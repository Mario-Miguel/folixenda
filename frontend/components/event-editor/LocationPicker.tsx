"use client";

import "leaflet/dist/leaflet.css";
import L from "leaflet";
import { useEffect } from "react";
import { MapContainer, Marker, TileLayer, useMap, useMapEvents } from "react-leaflet";

// Leaflet touches `window` on import: load this component with next/dynamic and `ssr: false`.

const ASTURIAS: [number, number] = [43.36, -5.85];
const DEFAULT_ZOOM = 9;
const PLACE_ZOOM = 16;

const pin = L.divIcon({
  className: "", // drop Leaflet's default white box
  html: `<svg width="34" height="46" viewBox="0 0 26 36" xmlns="http://www.w3.org/2000/svg">
    <path d="M13 0C5.8 0 0 5.8 0 13c0 9.4 13 23 13 23s13-13.6 13-23C26 5.8 20.2 0 13 0z"
          fill="#ec5b13" stroke="#ffffff" stroke-width="2"/>
    <circle cx="13" cy="13" r="4.5" fill="#ffffff"/>
  </svg>`,
  iconSize: [34, 46],
  iconAnchor: [17, 46], // tip of the pin on the coordinate
});

export interface LocationPickerProps {
  lat?: number;
  lon?: number;
  // Changes when the point was set from outside the map (address search): the map flies there
  focusKey: number;
  onPick: (lat: number, lon: number) => void;
  markerLabel: string;
}

function ClickToPick({ onPick }: { onPick: LocationPickerProps["onPick"] }) {
  useMapEvents({ click: (e) => onPick(e.latlng.lat, e.latlng.lng) });
  return null;
}

function FlyTo({ lat, lon, focusKey }: { lat?: number; lon?: number; focusKey: number }) {
  const map = useMap();
  useEffect(() => {
    if (focusKey > 0 && lat != null && lon != null) map.flyTo([lat, lon], Math.max(map.getZoom(), PLACE_ZOOM));
    // Only react to explicit focus requests, not to every click on the map
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [map, focusKey]);
  return null;
}

export default function LocationPicker({ lat, lon, focusKey, onPick, markerLabel }: LocationPickerProps) {
  const hasPoint = lat != null && lon != null;
  return (
    <MapContainer
      center={hasPoint ? [lat, lon] : ASTURIAS}
      zoom={hasPoint ? PLACE_ZOOM : DEFAULT_ZOOM}
      scrollWheelZoom
      className="h-full w-full cursor-crosshair"
    >
      <TileLayer
        attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
        url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
      />
      <ClickToPick onPick={onPick} />
      <FlyTo lat={lat} lon={lon} focusKey={focusKey} />
      {hasPoint && (
        <Marker
          position={[lat, lon]}
          icon={pin}
          draggable
          title={markerLabel}
          alt={markerLabel}
          eventHandlers={{
            dragend: (e) => {
              const p = (e.target as L.Marker).getLatLng();
              onPick(p.lat, p.lng);
            },
          }}
        />
      )}
    </MapContainer>
  );
}
