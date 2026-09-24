import { Map, List } from "lucide-react";

interface SwitchMapWidgetProps {
  showMap: boolean;
  setShowMap: (shoswMap: boolean) => void;
}

export default function SwitchMapWidget({ showMap, setShowMap }: SwitchMapWidgetProps) {
  return (
    <div className="flex rounded-xl border border-gray-200 overflow-hidden bg-white">
      <button
        onClick={() => setShowMap(false)}
        className={`flex-1 flex items-center justify-center gap-1.5 py-2.5 text-sm font-medium transition-colors ${
          !showMap ? "text-white" : "text-gray-600 hover:bg-gray-50"
        }`}
        style={!showMap ? { backgroundColor: "#ec5b13" } : undefined}
      >
        <List className="w-4 h-4" />
        Event list
      </button>
      <button
        onClick={() => setShowMap(true)}
        className={`flex-1 flex items-center justify-center gap-1.5 py-2.5 text-sm font-medium transition-colors ${
          showMap ? "text-white" : "text-gray-600 hover:bg-gray-50"
        }`}
        style={showMap ? { backgroundColor: "#ec5b13" } : undefined}
      >
        <Map className="w-4 h-4" />
        Map
      </button>
    </div>
  );
}
