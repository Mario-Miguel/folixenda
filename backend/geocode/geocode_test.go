package geocode

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// upstream is a programmable stand-in for CartoCiudad and Nominatim.
type upstream struct {
	mu       sync.Mutex
	handlers map[string]func(w http.ResponseWriter, r *http.Request) // by path
	calls    []string
	times    map[string][]time.Time
	agents   []string
}

func newUpstream(t *testing.T) (*upstream, *httptest.Server) {
	u := &upstream{handlers: map[string]func(http.ResponseWriter, *http.Request){}, times: map[string][]time.Time{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u.mu.Lock()
		u.calls = append(u.calls, r.URL.Path+"?"+r.URL.RawQuery)
		u.times[r.URL.Path] = append(u.times[r.URL.Path], time.Now())
		u.agents = append(u.agents, r.Header.Get("User-Agent"))
		h := u.handlers[r.URL.Path]
		u.mu.Unlock()
		if h == nil {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		h(w, r)
	}))
	t.Cleanup(srv.Close)
	return u, srv
}

func (u *upstream) on(path, body string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.handlers[path] = func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte(body)) } //nolint:errcheck
}

func (u *upstream) status(path string, code int) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.handlers[path] = func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(code) }
}

func (u *upstream) count(prefix string) int {
	u.mu.Lock()
	defer u.mu.Unlock()
	n := 0
	for _, c := range u.calls {
		if strings.HasPrefix(c, prefix) {
			n++
		}
	}
	return n
}

func newClient(srv *httptest.Server) *Client {
	return New(Options{CartoCiudadURL: srv.URL + "/carto/", NominatimURL: srv.URL + "/nomi", NominatimInterval: -1})
}

const (
	cartoUria = `[{"id":"1","type":"portal","address":"CALLE URIA 10, Oviedo/Uviéu (Oviedo)","tip_via":"CALLE","portalNumber":10,` +
		`"muni":"Oviedo","province":"Asturias","postalCode":"33003","lat":43.3626,"lng":-5.8489},` +
		`{"id":"1b","type":"portal","address":"CALLE URIA 10, Oviedo/Uviéu (Oviedo)","muni":"Oviedo","province":"Asturias","lat":43.3626,"lng":-5.8489},` +
		`{"id":"2","type":"toponimo","address":"Teatro Campoamor, Oviedo/Uviéu (Oviedo)","muni":"Oviedo","province":"Asturias","postalCode":"33002","lat":43.363,"lng":-5.8478}]`
	nomiSearch = `[{"lat":"43.36","lon":"-5.84","name":"","display_name":"10, Calle Uría, Oviedo, Asturias, 33003, España",` +
		`"address":{"road":"Calle Uría","house_number":"10","city":"Oviedo","province":"Asturias","state":"Principado de Asturias","postcode":"33003"}}]`
)

var (
	wantUria = Result{Label: "Calle Uria 10, Oviedo, Asturias", Address: "Calle Uria 10", Municipality: "Oviedo",
		Province: "Asturias", PostalCode: "33003", Lat: 43.3626, Lon: -5.8489}
	wantCampoamor = Result{Label: "Teatro Campoamor, Oviedo, Asturias", Address: "Teatro Campoamor", Municipality: "Oviedo",
		Province: "Asturias", PostalCode: "33002", Lat: 43.363, Lon: -5.8478}
	wantNomi = Result{Label: "Calle Uría 10, Oviedo, Asturias", Address: "Calle Uría 10", Municipality: "Oviedo",
		Province: "Asturias", PostalCode: "33003", Lat: 43.36, Lon: -5.84}
)

func TestSearchCartoCiudad(t *testing.T) {
	u, srv := newUpstream(t)
	u.on("/carto/candidates", cartoUria)
	c := newClient(srv)
	got, err := c.Search(context.Background(), "Calle Uria 10", 8)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []Result{wantUria, wantCampoamor}) {
		t.Fatalf("got %+v", got)
	}
	if u.calls[0] != "/carto/candidates?limit=8&q=Calle+Uria+10" || u.agents[0] != DefaultUserAgent {
		t.Fatalf("calls %v agents %v", u.calls, u.agents)
	}
	// Cached by normalized query: no new upstream call.
	if got2, _ := c.Search(context.Background(), "  calle   URIA 10 ", 8); !reflect.DeepEqual(got2, got) || len(u.calls) != 1 {
		t.Fatalf("cache miss: calls %v", u.calls)
	}
	// Different limit is a different key; and the limit cuts results.
	if got3, _ := c.Search(context.Background(), "calle uria 10", 1); len(got3) != 1 || len(u.calls) != 2 {
		t.Fatalf("got %v calls %v", got3, u.calls)
	}
}

func TestSearchFallsBackToNominatim(t *testing.T) {
	for _, tt := range []struct {
		name  string
		carto func(u *upstream)
	}{
		{"carto empty", func(u *upstream) { u.on("/carto/candidates", `[]`) }},
		{"carto 204", func(u *upstream) { u.status("/carto/candidates", 204) }},
		{"carto 500", func(u *upstream) { u.status("/carto/candidates", 500) }},
		{"carto bad json", func(u *upstream) { u.on("/carto/candidates", `{`) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			u, srv := newUpstream(t)
			tt.carto(u)
			u.on("/nomi/search", nomiSearch)
			got, err := newClient(srv).Search(context.Background(), "uria 10", 5)
			if err != nil || !reflect.DeepEqual(got, []Result{wantNomi}) {
				t.Fatalf("got %+v err %v", got, err)
			}
			if c := u.calls[len(u.calls)-1]; c != "/nomi/search?addressdetails=1&countrycodes=es&format=jsonv2&limit=5&q=uria+10" {
				t.Fatalf("nominatim call %s", c)
			}
		})
	}
	t.Run("nothing anywhere is an empty slice", func(t *testing.T) {
		u, srv := newUpstream(t)
		u.on("/carto/candidates", `[]`)
		u.on("/nomi/search", `[]`)
		got, err := newClient(srv).Search(context.Background(), "zzz", 5)
		if err != nil || got == nil || len(got) != 0 {
			t.Fatalf("got %v err %v", got, err)
		}
	})
	t.Run("nominatim 204 is empty", func(t *testing.T) {
		u, srv := newUpstream(t)
		u.on("/carto/candidates", `[]`)
		u.status("/nomi/search", 204)
		if got, err := newClient(srv).Search(context.Background(), "zzz", 5); err != nil || got == nil || len(got) != 0 {
			t.Fatalf("got %v err %v", got, err)
		}
	})
	t.Run("both fail: error, not cached", func(t *testing.T) {
		u, srv := newUpstream(t)
		c := newClient(srv)
		if _, err := c.Search(context.Background(), "zzz", 5); err == nil {
			t.Fatal("expected error")
		}
		u.on("/nomi/search", `{`)
		if _, err := c.Search(context.Background(), "zzz", 5); err == nil || !strings.Contains(err.Error(), "nominatim search") {
			t.Fatalf("err %v", err)
		}
		if u.count("/nomi/search") != 2 {
			t.Fatalf("errors must not be cached: %v", u.calls)
		}
	})
	t.Run("nominatim places without coordinates are skipped", func(t *testing.T) {
		u, srv := newUpstream(t)
		u.on("/carto/candidates", `[]`)
		u.on("/nomi/search", `[{"lat":"x","lon":"1"},`+nomiSearch[1:len(nomiSearch)-1]+`,`+nomiSearch[1:])
		got, _ := newClient(srv).Search(context.Background(), "zzz", 5)
		if len(got) != 1 {
			t.Fatalf("got %+v", got)
		}
	})
}

func TestSearchResolvesCandidatesWithoutCoordinates(t *testing.T) {
	u, srv := newUpstream(t)
	u.on("/carto/candidates", `[{"type":"callejero","address":"CALLE URIA, Oviedo","muni":"Oviedo","province":"Asturias","lat":null,"lng":null},`+
		`{"type":"Municipio","address":"Gijón","muni":"Gijón","province":"Asturias","lat":0,"lng":0},`+
		`{"type":"x","address":"No Match","lat":null,"lng":null},`+
		`{"type":"x","address":"Bad","lat":null,"lng":null},`+
		`{"type":"x","address":"Broken","lat":null,"lng":null},`+
		`{"type":"x","address":"NoCoords","lat":null,"lng":null},`+
		`{"type":"x","address":"Fails","lat":null,"lng":null}]`)
	u.mu.Lock()
	u.handlers["/carto/findJsonp"] = func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("q") {
		case "CALLE URIA, Oviedo":
			w.Write([]byte(`callback({"lat":43.36,"lng":-5.85})`)) //nolint:errcheck
		case "Gijón":
			w.Write([]byte(`callback({"lat":43.53,"lng":-5.66})`)) //nolint:errcheck
		case "No Match":
			w.Write([]byte(`callback([])`)) //nolint:errcheck
		case "Bad":
			w.Write([]byte(`not jsonp`)) //nolint:errcheck
		case "Broken":
			w.Write([]byte(`callback({)`)) //nolint:errcheck
		case "NoCoords":
			w.Write([]byte(`callback({"lat":null})`)) //nolint:errcheck
		default:
			w.WriteHeader(500)
		}
	}
	u.mu.Unlock()
	got, err := newClient(srv).Search(context.Background(), "uria", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Address != "Calle Uria" || got[0].Lat != 43.36 || got[1].Label != "Gijón, Asturias" || got[1].Lon != -5.66 {
		t.Fatalf("got %+v", got)
	}
}

func TestReverse(t *testing.T) {
	ctx := context.Background()
	t.Run("cartociudad", func(t *testing.T) {
		u, srv := newUpstream(t)
		u.on("/carto/reverseGeocode", `{"type":"portal","address":"URIA","tip_via":"CALLE","portalNumber":10,"muni":"Oviedo",`+
			`"province":"Asturias","postalCode":"33003","lat":43.3626,"lng":-5.8489}`)
		c := newClient(srv)
		got, err := c.Reverse(ctx, 43.3626, -5.8489)
		if err != nil || *got != wantUria {
			t.Fatalf("got %+v err %v", got, err)
		}
		if u.calls[0] != "/carto/reverseGeocode?lat=43.3626&lon=-5.8489" {
			t.Fatalf("call %s", u.calls[0])
		}
		if again, _ := c.Reverse(ctx, 43.362601, -5.848901); *again != wantUria || len(u.calls) != 1 {
			t.Fatalf("nearby point should hit the cache: %v", u.calls)
		}
	})
	t.Run("street without number", func(t *testing.T) {
		u, srv := newUpstream(t)
		u.on("/carto/reverseGeocode", `{"address":"Plaza de la Catedral","muni":"Oviedo","lat":1,"lng":2,"portalNumber":0}`)
		got, err := newClient(srv).Reverse(ctx, 1, 2)
		if err != nil || got.Address != "Plaza de la Catedral" || got.Label != "Plaza de la Catedral, Oviedo" {
			t.Fatalf("got %+v err %v", got, err)
		}
	})
	t.Run("carto 204 → nominatim", func(t *testing.T) {
		u, srv := newUpstream(t)
		u.status("/carto/reverseGeocode", 204)
		u.on("/nomi/reverse", nomiSearch[1:len(nomiSearch)-1])
		got, err := newClient(srv).Reverse(ctx, 43.36, -5.84)
		if err != nil || *got != wantNomi {
			t.Fatalf("got %+v err %v", got, err)
		}
		if c := u.calls[1]; c != "/nomi/reverse?addressdetails=1&format=jsonv2&lat=43.36&lon=-5.84" {
			t.Fatalf("call %s", c)
		}
	})
	t.Run("carto without address → nominatim error → not found, cached", func(t *testing.T) {
		u, srv := newUpstream(t)
		u.on("/carto/reverseGeocode", `{"address":"","lat":1,"lng":1}`)
		u.on("/nomi/reverse", `{"error":"Unable to geocode"}`)
		c := newClient(srv)
		if _, err := c.Reverse(ctx, 1, 1); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err %v", err)
		}
		if _, err := c.Reverse(ctx, 1, 1); !errors.Is(err, ErrNotFound) || len(u.calls) != 2 {
			t.Fatalf("err %v calls %v", err, u.calls)
		}
	})
	t.Run("nominatim place without coordinates → not found", func(t *testing.T) {
		u, srv := newUpstream(t)
		u.on("/carto/reverseGeocode", `{`)
		u.on("/nomi/reverse", `{"lat":"","lon":""}`)
		if _, err := newClient(srv).Reverse(ctx, 1, 1); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err %v", err)
		}
	})
	t.Run("nominatim 204 → not found", func(t *testing.T) {
		u, srv := newUpstream(t)
		u.status("/carto/reverseGeocode", 204)
		u.status("/nomi/reverse", 204)
		if _, err := newClient(srv).Reverse(ctx, 1, 1); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err %v", err)
		}
	})
	t.Run("both fail", func(t *testing.T) {
		u, srv := newUpstream(t)
		u.on("/nomi/reverse", `{`)
		_, err := newClient(srv).Reverse(ctx, 1, 1)
		if err == nil || errors.Is(err, ErrNotFound) || !strings.Contains(err.Error(), "nominatim reverse") {
			t.Fatalf("err %v", err)
		}
	})
}

func TestNominatimIsRateLimited(t *testing.T) {
	u, srv := newUpstream(t)
	u.on("/carto/candidates", `[]`)
	u.on("/nomi/search", `[]`)
	c := New(Options{CartoCiudadURL: srv.URL + "/carto", NominatimURL: srv.URL + "/nomi", NominatimInterval: 80 * time.Millisecond})
	var wg sync.WaitGroup
	for _, q := range []string{"aaa", "bbb", "ccc"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Search(context.Background(), q, 1) //nolint:errcheck
		}()
	}
	wg.Wait()
	times := u.times["/nomi/search"]
	if len(times) != 3 {
		t.Fatalf("calls %v", u.calls)
	}
	for i := 1; i < len(times); i++ {
		if gap := times[i].Sub(times[i-1]); gap < 70*time.Millisecond {
			t.Fatalf("nominatim calls %v apart", gap)
		}
	}

	t.Run("waiting honours cancellation", func(t *testing.T) {
		c.lastNominatim = time.Now()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := c.nominatimGet(ctx, "/search", nil); !errors.Is(err, context.Canceled) {
			t.Fatalf("err %v", err)
		}
	})
	t.Run("no wait once the interval passed", func(t *testing.T) {
		c.lastNominatim = time.Now().Add(-time.Second)
		if err := c.waitNominatim(context.Background()); err != nil {
			t.Fatal(err)
		}
	})
}

func TestGetErrors(t *testing.T) {
	c := New(Options{})
	if _, err := c.get(context.Background(), "http://bad host", nil); err == nil {
		t.Fatal("bad URL must fail")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.Write([]byte("short")) //nolint:errcheck
	}))
	if _, err := c.get(context.Background(), srv.URL, nil); err == nil {
		t.Fatal("truncated body must fail")
	}
	srv.Close()
	if _, err := c.get(context.Background(), srv.URL, nil); err == nil {
		t.Fatal("closed server must fail")
	}
}

func TestDefaults(t *testing.T) {
	c := New(Options{})
	if c.cartoURL != DefaultCartoCiudadURL || c.nominatimURL != DefaultNominatimURL || c.userAgent != DefaultUserAgent ||
		c.interval != NominatimInterval || c.cache.ttl != CacheTTL || c.cache.max != CacheSize || c.http.Timeout == 0 {
		t.Fatalf("defaults %+v", c)
	}
	h := &http.Client{}
	c = New(Options{UserAgent: "x", HTTPClient: h, CacheTTL: time.Second, CacheSize: 2})
	if c.userAgent != "x" || c.http != h || c.cache.ttl != time.Second || c.cache.max != 2 {
		t.Fatalf("options %+v", c)
	}
}

func TestPrettyStreetAndLabel(t *testing.T) {
	for in, want := range map[string]string{
		"CALLE URIA 10":                "Calle Uria 10",
		"AVENIDA DE LA CONSTITUCIÓN 3": "Avenida de la Constitución 3",
		"Teatro Campoamor":             "Teatro Campoamor",
		"":                             "",
		"ÉL Y LOS DEMÁS":               "Él y los Demás",
	} {
		if got := prettyStreet(in); got != want {
			t.Errorf("prettyStreet(%q) = %q, want %q", in, got, want)
		}
	}
	if got := label("Oviedo", " oviedo ", "", "Asturias"); got != "Oviedo, Asturias" {
		t.Errorf("label = %q", got)
	}
}

func TestNominatimResultFallbacks(t *testing.T) {
	p := nominatimPlace{Lat: "1", Lon: "2", Name: "Playa de San Lorenzo", Address: map[string]string{"town": "Gijón", "county": "Asturias"}}
	r, ok := p.result()
	if !ok || r.Address != "Playa de San Lorenzo" || r.Municipality != "Gijón" || r.Province != "Asturias" {
		t.Fatalf("got %+v", r)
	}
	p = nominatimPlace{Lat: "1", Lon: "2", Name: "Teatro Campoamor", Address: map[string]string{"road": "Calle Pelayo", "city": "Oviedo"}}
	if r, _ := p.result(); r.Address != "Calle Pelayo" || r.Label != "Teatro Campoamor, Calle Pelayo, Oviedo" {
		t.Fatalf("got %+v", r)
	}
	p = nominatimPlace{Lat: "1", Lon: "2", DisplayName: "Somewhere, Else"}
	if r, _ := p.result(); r.Address != "Somewhere" || r.Label != "Somewhere" {
		t.Fatalf("got %+v", r)
	}
	if _, ok := (nominatimPlace{Lat: "1", Lon: "x"}).result(); ok {
		t.Fatal("bad lon must fail")
	}
}

func TestCache(t *testing.T) {
	now := time.Unix(0, 0)
	c := newTTLCache(time.Minute, 2, func() time.Time { return now })
	c.put("a", 1)
	now = now.Add(time.Second)
	c.put("b", 2)
	c.put("b", 3) // update in place: no eviction
	if v, ok := c.get("a"); !ok || v != 1 {
		t.Fatal("a missing")
	}
	now = now.Add(time.Second)
	c.put("c", 4) // full: evicts the oldest (a)
	if _, ok := c.get("a"); ok {
		t.Fatal("a should be evicted")
	}
	if v, _ := c.get("b"); v != 3 {
		t.Fatalf("b = %v", v)
	}
	now = now.Add(2 * time.Minute) // everything expired
	if _, ok := c.get("b"); ok {
		t.Fatal("b should be expired")
	}
	c.put("d", 5) // c expired: dropped, no live eviction needed
	c.put("e", 6)
	if _, ok := c.get("d"); !ok {
		t.Fatal("d missing")
	}
	if _, ok := c.get("e"); !ok {
		t.Fatal("e missing")
	}
}
