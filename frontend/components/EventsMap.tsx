"use client";

import "leaflet/dist/leaflet.css";
import L from "leaflet";
import { useEffect } from "react";
import { Circle, MapContainer, Marker, TileLayer, Tooltip, useMap } from "react-leaflet";

// Leaflet touches `window` on import: load this component with next/dynamic and `ssr: false`.

const OVIEDO: [number, number] = [43.3614, -5.8494];
const DEFAULT_ZOOM = 13;
const SELECTED_ZOOM = 13;
const SELECTED_COLOR = "#ec5b13";
const DEFAULT_COLOR = "#6b7280";
// Radius (meters) of the area drawn for events without a precise location
const IMPRECISE_RADIUS_M = 400;

// Icons are cached so react-leaflet doesn't call setIcon on every render
const pinIcons = new Map<string, L.DivIcon>();

function pinIcon(color: string, selected: boolean): L.DivIcon {
  const key = `${color}-${selected}`;
  let icon = pinIcons.get(key);
  if (!icon) {
    const [w, h] = selected ? [34, 46] : [26, 36];
    icon = L.divIcon({
      className: "", // drop Leaflet's default white box
      html: `<svg width="${w}" height="${h}" viewBox="0 0 26 36" xmlns="http://www.w3.org/2000/svg">
        <path d="M13 0C5.8 0 0 5.8 0 13c0 9.4 13 23 13 23s13-13.6 13-23C26 5.8 20.2 0 13 0z"
              fill="${color}" stroke="#ffffff" stroke-width="2"/>
        <circle cx="13" cy="13" r="4.5" fill="#ffffff"/>
      </svg>`,
      iconSize: [w, h],
      iconAnchor: [w / 2, h], // tip of the pin sits on the coordinate
      tooltipAnchor: [0, -h],
    });
    pinIcons.set(key, icon);
  }
  return icon;
}

export interface MapMarker {
  id: string;
  lat: number;
  lon: number;
  // false when lat/lon is an approximation: drawn as an area circle instead of a pin
  precise: boolean;
  title: string;
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
        const color = isSelected ? SELECTED_COLOR : (m.color ?? DEFAULT_COLOR);
        const handlers = { click: () => onSelect?.(isSelected ? null : m.id) };
        const tooltip = (
          <Tooltip direction="top" permanent={isSelected}>
            <span className="font-semibold">{m.title}</span>
            {!m.precise && <span className="block text-gray-500">Approximate location</span>}
          </Tooltip>
        );

        if (m.precise) {
          return (
            <Marker
              key={m.id}
              position={[m.lat, m.lon]}
              icon={pinIcon(color, isSelected)}
              zIndexOffset={isSelected ? 1000 : 0}
              eventHandlers={handlers}
            >
              {tooltip}
            </Marker>
          );
        }

        return (
          <Circle
            key={m.id}
            center={[m.lat, m.lon]}
            radius={IMPRECISE_RADIUS_M}
            pathOptions={{
              color,
              weight: isSelected ? 3 : 1.5,
              dashArray: "6 4",
              fillColor: color,
              fillOpacity: isSelected ? 0.3 : 0.15,
            }}
            eventHandlers={handlers}
          >
            {tooltip}
          </Circle>
        );
      })}

      <FlyToSelected marker={selected} />
    </MapContainer>
  );
}
