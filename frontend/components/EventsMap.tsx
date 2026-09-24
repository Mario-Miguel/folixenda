"use client";

import "leaflet/dist/leaflet.css";
import { useEffect } from "react";
import { CircleMarker, MapContainer, TileLayer, Tooltip, useMap } from "react-leaflet";

// Leaflet touches `window` on import: load this component with next/dynamic and `ssr: false`.

const OVIEDO: [number, number] = [43.3614, -5.8494];
const DEFAULT_ZOOM = 13;
const SELECTED_ZOOM = 15;

export interface MapMarker {
  id: string;
  lat: number;
  lon: number;
  title: string;
  subtitle?: string;
  color?: string;
}

interface EventsMapProps {
  markers?: MapMarker[];
  selectedId?: string | null;
  onSelect?: (id: string | null) => void;
}

function FlyToSelected({ marker }: { marker?: MapMarker }) {
  const map = useMap();

  useEffect(() => {
    if (marker) map.flyTo([marker.lat, marker.lon], Math.max(map.getZoom(), SELECTED_ZOOM));
  }, [map, marker]);

  return null;
}

export default function EventsMap({ markers = [], selectedId = null, onSelect }: EventsMapProps) {
  const selected = markers.find((m) => m.id === selectedId);

  return (
    <MapContainer center={OVIEDO} zoom={DEFAULT_ZOOM} scrollWheelZoom className="h-full w-full">
      <TileLayer
        attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
        url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
      />

      {markers.map((m) => {
        const isSelected = m.id === selectedId;
        const color = isSelected ? "#ec5b13" : (m.color ?? "#6b7280");

        return (
          <CircleMarker
            key={m.id}
            center={[m.lat, m.lon]}
            radius={isSelected ? 11 : 7}
            pathOptions={{ color: "#ffffff", weight: 2, fillColor: color, fillOpacity: 1 }}
            eventHandlers={{ click: () => onSelect?.(isSelected ? null : m.id) }}
          >
            <Tooltip direction="top" offset={[0, -8]} permanent={isSelected}>
              <span className="font-semibold">{m.title}</span>
              {m.subtitle && <span className="block text-gray-500">{m.subtitle}</span>}
            </Tooltip>
          </CircleMarker>
        );
      })}

      <FlyToSelected marker={selected} />
    </MapContainer>
  );
}
