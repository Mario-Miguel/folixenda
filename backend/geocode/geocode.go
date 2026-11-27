// Package geocode turns addresses into coordinates and back, using
// CartoCiudad (IGN/CNIG, very precise for Spain) first and Nominatim
// (OpenStreetMap) as the fallback.
//
// Usage policies: requests carry a generic User-Agent (no personal data),
// Nominatim calls are serialized to at most one per NominatimInterval, and
// results are cached in memory for CacheTTL.
package geocode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Result is one geocoded place.
type Result struct {
	Label        string  `json:"label"`   // full display text
	Address      string  `json:"address"` // street + number (or place name)
	Municipality string  `json:"municipality,omitempty"`
	Province     string  `json:"province,omitempty"`
	PostalCode   string  `json:"postalCode,omitempty"`
	Lat          float64 `json:"lat"`
	Lon          float64 `json:"lon"`
}

// ErrNotFound: no provider found anything at that point (Reverse).
var ErrNotFound = errors.New("no address found")

const (
	DefaultCartoCiudadURL = "https://www.cartociudad.es/geocoder/api/geocoder"
	DefaultNominatimURL   = "https://nominatim.openstreetmap.org"
	DefaultUserAgent      = "folixenda/1.0 (+https://github.com/Mario-Miguel/folixenda)"
	// NominatimInterval: Nominatim's policy allows at most 1 request/second.
	NominatimInterval = 1100 * time.Millisecond
	CacheTTL          = 10 * time.Minute
	CacheSize         = 1000
)

// Options configures a Client. Zero values take the defaults above.
type Options struct {
	CartoCiudadURL    string
	NominatimURL      string
	UserAgent         string
	HTTPClient        *http.Client
	NominatimInterval time.Duration // negative: no spacing (tests)
	CacheTTL          time.Duration
	CacheSize         int
}

// Client geocodes with CartoCiudad, falling back to Nominatim. Safe for
// concurrent use.
type Client struct {
	cartoURL, nominatimURL, userAgent string
	http                              *http.Client
	interval                          time.Duration

	nominatimMu   sync.Mutex // held during a Nominatim call: serializes them
	lastNominatim time.Time

	cache *ttlCache
	now   func() time.Time
}

func New(o Options) *Client {
	c := &Client{
		cartoURL:     strings.TrimRight(cmpOr(o.CartoCiudadURL, DefaultCartoCiudadURL), "/"),
		nominatimURL: strings.TrimRight(cmpOr(o.NominatimURL, DefaultNominatimURL), "/"),
		userAgent:    cmpOr(o.UserAgent, DefaultUserAgent),
		http:         o.HTTPClient,
		interval:     o.NominatimInterval,
		now:          time.Now,
	}
	if c.http == nil {
		c.http = &http.Client{Timeout: 8 * time.Second}
	}
	if c.interval == 0 {
		c.interval = NominatimInterval
	}
	ttl, size := o.CacheTTL, o.CacheSize
	if ttl == 0 {
		ttl = CacheTTL
	}
	if size == 0 {
		size = CacheSize
	}
	c.cache = newTTLCache(ttl, size, func() time.Time { return c.now() })
	return c
}

func cmpOr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// normalizeQuery lowercases q and collapses its whitespace, for cache keys.
func normalizeQuery(q string) string {
	return strings.ToLower(strings.Join(strings.Fields(q), " "))
}

// Search returns up to limit places matching q. An empty slice (not an
// error) means nothing was found.
func (c *Client) Search(ctx context.Context, q string, limit int) ([]Result, error) {
	key := fmt.Sprintf("s|%d|%s", limit, normalizeQuery(q))
	if v, ok := c.cache.get(key); ok {
		return v.([]Result), nil
	}
	results, cartoErr := c.cartoSearch(ctx, q, limit)
	if cartoErr != nil || len(results) == 0 {
		var err error
		results, err = c.nominatimSearch(ctx, q, limit)
		if err != nil {
			return nil, errors.Join(cartoErr, err)
		}
	}
	c.cache.put(key, results)
	return results, nil
}

// Reverse returns the address at a point, or ErrNotFound.
func (c *Client) Reverse(ctx context.Context, lat, lon float64) (*Result, error) {
	key := fmt.Sprintf("r|%.5f|%.5f", lat, lon)
	if v, ok := c.cache.get(key); ok {
		if v == nil {
			return nil, ErrNotFound
		}
		r := v.(Result)
		return &r, nil
	}
	res, cartoErr := c.cartoReverse(ctx, lat, lon)
	if cartoErr != nil || res == nil {
		var err error
		res, err = c.nominatimReverse(ctx, lat, lon)
		if err != nil {
			return nil, errors.Join(cartoErr, err)
		}
	}
	if res == nil {
		c.cache.put(key, nil)
		return nil, ErrNotFound
	}
	c.cache.put(key, *res)
	return res, nil
}

// get fetches base?params and returns the body of a 2xx response; a 204 is
// (nil, nil).
func (c *Client) get(ctx context.Context, base string, params url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "es")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("%s: HTTP %d", base, resp.StatusCode)
	}
	if resp.StatusCode == http.StatusNoContent || len(strings.TrimSpace(string(body))) == 0 {
		return nil, nil
	}
	return body, nil
}

// waitNominatim must be called with nominatimMu held: it sleeps until the
// interval since the previous Nominatim call has passed.
func (c *Client) waitNominatim(ctx context.Context) error {
	if c.interval < 0 || c.lastNominatim.IsZero() {
		return nil
	}
	wait := c.interval - c.now().Sub(c.lastNominatim)
	if wait <= 0 {
		return nil
	}
	t := time.NewTimer(wait)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// nominatimGet runs one rate-limited Nominatim request.
func (c *Client) nominatimGet(ctx context.Context, path string, params url.Values) ([]byte, error) {
	c.nominatimMu.Lock()
	defer c.nominatimMu.Unlock()
	if err := c.waitNominatim(ctx); err != nil {
		return nil, err
	}
	defer func() { c.lastNominatim = c.now() }()
	return c.get(ctx, c.nominatimURL+path, params)
}

// label joins the non-empty, distinct parts with ", ".
func label(parts ...string) string {
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		dup := false
		for _, o := range out {
			if strings.EqualFold(o, p) {
				dup = true
			}
		}
		if !dup {
			out = append(out, p)
		}
	}
	return strings.Join(out, ", ")
}

// lowerWords stay lowercase inside title-cased street names.
var lowerWords = map[string]bool{"de": true, "del": true, "la": true, "las": true, "los": true, "el": true, "y": true, "e": true, "a": true}

// prettyStreet title-cases an ALL-CAPS street ("CALLE URIA 10" → "Calle Uria
// 10"); anything with lowercase letters is left as is.
func prettyStreet(s string) string {
	if s != strings.ToUpper(s) {
		return s
	}
	words := strings.Fields(strings.ToLower(s))
	for i, w := range words {
		if i > 0 && lowerWords[w] {
			continue
		}
		r := []rune(w)
		r[0] = []rune(strings.ToUpper(string(r[0])))[0]
		words[i] = string(r)
	}
	return strings.Join(words, " ")
}

// ---- CartoCiudad ----

type cartoCandidate struct {
	ID           string   `json:"id"`
	Type         string   `json:"type"`
	Address      string   `json:"address"`
	TipVia       string   `json:"tip_via"`
	PortalNumber *int     `json:"portalNumber"`
	Muni         string   `json:"muni"`
	Poblacion    string   `json:"poblacion"`
	Province     string   `json:"province"`
	PostalCode   string   `json:"postalCode"`
	Lat          *float64 `json:"lat"`
	Lng          *float64 `json:"lng"`
}

func (c cartoCandidate) hasCoords() bool {
	return c.Lat != nil && c.Lng != nil && (*c.Lat != 0 || *c.Lng != 0)
}

func (c cartoCandidate) result(address string) Result {
	address = prettyStreet(strings.TrimSpace(address))
	return Result{
		Label:        label(address, c.Muni, c.Province),
		Address:      address,
		Municipality: c.Muni,
		Province:     c.Province,
		PostalCode:   c.PostalCode,
		Lat:          *c.Lat,
		Lon:          *c.Lng,
	}
}

// candidateAddress is the street part of a candidates "address", which looks
// like "CALLE URIA 10, Oviedo/Uviéu (Oviedo)".
func candidateAddress(a string) string {
	if i := strings.Index(a, ", "); i > 0 {
		return a[:i]
	}
	return a
}

func (c *Client) cartoSearch(ctx context.Context, q string, limit int) ([]Result, error) {
	body, err := c.get(ctx, c.cartoURL+"/candidates", url.Values{"q": {q}, "limit": {strconv.Itoa(limit)}})
	if err != nil || body == nil {
		return nil, err
	}
	var cands []cartoCandidate
	if err := json.Unmarshal(body, &cands); err != nil {
		return nil, fmt.Errorf("cartociudad candidates: %w", err)
	}
	results := []Result{}
	seen := map[string]bool{}
	for _, cand := range cands {
		if !cand.hasCoords() {
			// Some candidate types come without coordinates: resolve them
			// with find, as the geocoder skill does.
			found, err := c.cartoFind(ctx, cand.Address)
			if err != nil || found == nil {
				continue
			}
			cand.Lat, cand.Lng = found.Lat, found.Lng
		}
		r := cand.result(candidateAddress(cand.Address))
		if seen[r.Label] {
			continue
		}
		seen[r.Label] = true
		results = append(results, r)
		if len(results) == limit {
			break
		}
	}
	return results, nil
}

// cartoFind geocodes q to one candidate with findJsonp (JSONP, as in the
// geocoder skill): callback({...}) or callback([]) when nothing is found.
func (c *Client) cartoFind(ctx context.Context, q string) (*cartoCandidate, error) {
	body, err := c.get(ctx, c.cartoURL+"/findJsonp", url.Values{"q": {q}})
	if err != nil || body == nil {
		return nil, err
	}
	s := string(body)
	start, end := strings.Index(s, "("), strings.LastIndex(s, ")")
	if start == -1 || end <= start {
		return nil, errors.New("cartociudad find: not JSONP")
	}
	inner := strings.TrimSpace(s[start+1 : end])
	if strings.HasPrefix(inner, "[") {
		return nil, nil
	}
	var cand cartoCandidate
	if err := json.Unmarshal([]byte(inner), &cand); err != nil {
		return nil, fmt.Errorf("cartociudad find: %w", err)
	}
	if !cand.hasCoords() {
		return nil, nil
	}
	return &cand, nil
}

func (c *Client) cartoReverse(ctx context.Context, lat, lon float64) (*Result, error) {
	body, err := c.get(ctx, c.cartoURL+"/reverseGeocode", url.Values{
		"lat": {strconv.FormatFloat(lat, 'f', -1, 64)}, "lon": {strconv.FormatFloat(lon, 'f', -1, 64)}})
	if err != nil || body == nil {
		return nil, err
	}
	var cand cartoCandidate
	if err := json.Unmarshal(body, &cand); err != nil {
		return nil, fmt.Errorf("cartociudad reverse: %w", err)
	}
	if !cand.hasCoords() || cand.Address == "" {
		return nil, nil
	}
	// Reverse gives the street name alone: "URIA" + tip_via "CALLE" + portal 10.
	address := strings.TrimSpace(cand.TipVia + " " + cand.Address)
	if cand.PortalNumber != nil && *cand.PortalNumber > 0 {
		address += " " + strconv.Itoa(*cand.PortalNumber)
	}
	r := cand.result(address)
	return &r, nil
}

// ---- Nominatim ----

type nominatimPlace struct {
	Lat         string            `json:"lat"`
	Lon         string            `json:"lon"`
	Name        string            `json:"name"`
	DisplayName string            `json:"display_name"`
	Address     map[string]string `json:"address"`
}

func firstOf(m map[string]string, keys ...string) string {
	for _, k := range keys {
		if v := m[k]; v != "" {
			return v
		}
	}
	return ""
}

func (p nominatimPlace) result() (Result, bool) {
	lat, err1 := strconv.ParseFloat(p.Lat, 64)
	lon, err2 := strconv.ParseFloat(p.Lon, 64)
	if err1 != nil || err2 != nil {
		return Result{}, false
	}
	a := p.Address
	street := firstOf(a, "road", "pedestrian", "square", "footway", "path", "place")
	if street != "" {
		if n := a["house_number"]; n != "" {
			street += " " + n
		}
	} else {
		street = p.Name
	}
	if street == "" {
		street, _, _ = strings.Cut(p.DisplayName, ",")
	}
	r := Result{
		Address:      strings.TrimSpace(street),
		Municipality: firstOf(a, "city", "town", "village", "municipality", "hamlet"),
		Province:     firstOf(a, "province", "county", "state"),
		PostalCode:   a["postcode"],
		Lat:          lat,
		Lon:          lon,
	}
	// A named place (theatre, park…) leads the label; address stays the street.
	r.Label = label(p.Name, r.Address, r.Municipality, r.Province)
	return r, true
}

func (c *Client) nominatimSearch(ctx context.Context, q string, limit int) ([]Result, error) {
	body, err := c.nominatimGet(ctx, "/search", url.Values{"q": {q}, "format": {"jsonv2"}, "addressdetails": {"1"},
		"countrycodes": {"es"}, "limit": {strconv.Itoa(limit)}})
	if err != nil {
		return nil, err
	}
	results := []Result{}
	if body == nil {
		return results, nil
	}
	var places []nominatimPlace
	if err := json.Unmarshal(body, &places); err != nil {
		return nil, fmt.Errorf("nominatim search: %w", err)
	}
	seen := map[string]bool{}
	for _, p := range places {
		if r, ok := p.result(); ok && !seen[r.Label] {
			seen[r.Label] = true
			results = append(results, r)
		}
	}
	return results, nil
}

func (c *Client) nominatimReverse(ctx context.Context, lat, lon float64) (*Result, error) {
	body, err := c.nominatimGet(ctx, "/reverse", url.Values{"lat": {strconv.FormatFloat(lat, 'f', -1, 64)},
		"lon": {strconv.FormatFloat(lon, 'f', -1, 64)}, "format": {"jsonv2"}, "addressdetails": {"1"}})
	if err != nil || body == nil {
		return nil, err
	}
	var place struct {
		nominatimPlace
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &place); err != nil {
		return nil, fmt.Errorf("nominatim reverse: %w", err)
	}
	if place.Error != "" {
		return nil, nil // "Unable to geocode"
	}
	r, ok := place.result()
	if !ok {
		return nil, nil
	}
	return &r, nil
}
