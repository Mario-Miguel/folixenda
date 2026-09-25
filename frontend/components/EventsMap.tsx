"use client";

import "leaflet/dist/leaflet.css";
import L from "leaflet";
import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Circle, MapContainer, Marker, Popup, TileLayer, useMap } from "react-leaflet";
import EventCard from "@/components/EventCard";
import type { Event } from "@/lib/types";

// Leaflet touches `window` on import: load this component with next/dynamic and `ssr: false`.

const OVIEDO: [number, number] = [43.3614, -5.8494];
const DEFAULT_ZOOM = 13;
const SELECTED_ZOOM = 13;
const SELECTED_COLOR = "#ec5b13";
const DEFAULT_COLOR = "#6b7280";
// Radius (meters) of the area drawn for events without a precise location
const IMPRECISE_RADIUS_M = 400;
// Delay before hiding the hover card, so the cursor can travel from the pin to the card
const HOVER_CLOSE_DELAY_MS = 250;

function pinSize(selected: boolean): [number, number] {
  return selected ? [34, 46] : [26, 36];
}

// Icons are cached so react-leaflet doesn't call setIcon on every render
const pinIcons = new Map<string, L.DivIcon>();

function pinIcon(color: string, selected: boolean): L.DivIcon {
  const key = `${color}-${selected}`;
  let icon = pinIcons.get(key);
  if (!icon) {
    const [w, h] = pinSize(selected);
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
  color?: string;
  event: Event; // shown in the hover/selected card
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
  const { t } = useTranslation();
  const selected = markers.find((m) => m.id === selectedId);
  const [hoveredId, setHoveredId] = useState<string | null>(null);
  const hovered = markers.find((m) => m.id === hoveredId);
  const closeTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const cancelClose = () => {
    if (closeTimer.current) clearTimeout(closeTimer.current);
    closeTimer.current = null;
  };
  const showCard = (id: string) => {
    cancelClose();
    setHoveredId(id);
  };
  const scheduleClose = () => {
    cancelClose();
    closeTimer.current = setTimeout(() => setHoveredId(null), HOVER_CLOSE_DELAY_MS);
  };

  useEffect(() => cancelClose, []);

  // Selected event keeps its card open; hovering another event shows its card on top
  const renderCard = (m: MapMarker, isSelected: boolean) => (
    <Popup
      key={`${m.id}-${isSelected ? "selected" : "hover"}`}
      position={[m.lat, m.lon]}
      // Lift the card above the pin; area circles open it at their center.
      // Options are fixed at creation, hence the key including the selected state.
      offset={m.precise ? [0, 7 - pinSize(isSelected)[1]] : [0, 7]}
      closeButton={false}
      autoPan={false}
      autoClose={false} // let the selected and hovered cards coexist
      closeOnClick={!isSelected}
      closeOnEscapeKey={!isSelected}
      minWidth={240}
      maxWidth={240}
      className="event-map-popup"
      eventHandlers={isSelected ? undefined : { remove: () => setHoveredId((cur) => (cur === m.id ? null : cur)) }}
    >
      <div className="w-60" onMouseEnter={cancelClose} onMouseLeave={scheduleClose}>
        <EventCard event={m.event} detailed={false} approximateLocation={!m.precise} />
      </div>
    </Popup>
  );

  return (
    <MapContainer center={OVIEDO} zoom={DEFAULT_ZOOM} scrollWheelZoom className="h-full w-full">
      <TileLayer
        attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
        url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
      />

      {markers.map((m) => {
        const isSelected = m.id === selectedId;
        const color = isSelected ? SELECTED_COLOR : (m.color ?? DEFAULT_COLOR);
        const handlers = {
          click: () => {
            onSelect?.(isSelected ? null : m.id);
            showCard(m.id); // touch devices have no hover
          },
          mouseover: () => showCard(m.id),
          mouseout: scheduleClose,
        };

        if (m.precise) {
          return (
            <Marker
              key={m.id}
              position={[m.lat, m.lon]}
              icon={pinIcon(color, isSelected)}
              zIndexOffset={isSelected ? 1000 : 0}
              eventHandlers={handlers}
            />
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
          />
        );
      })}

      {selected && renderCard(selected, true)}
      {hovered && hovered.id !== selectedId && renderCard(hovered, false)}

      <FlyToSelected marker={selected} />
    </MapContainer>
  );
}
