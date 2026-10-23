import { Tab, TabGroup, TabList, TabPanel, TabPanels } from "@headlessui/react";
import type { ReactNode } from "react";

export type TabItem = { id: string; label: ReactNode; content: ReactNode; disabled?: boolean };
export type TabsProps = {
  items: TabItem[];
  selectedIndex?: number;
  onChange?: (index: number) => void;
  label?: string;
  className?: string;
};
export function Tabs({
  items,
  selectedIndex,
  onChange,
  label = "Sections",
  className = "",
}: TabsProps) {
  return (
    <TabGroup selectedIndex={selectedIndex} onChange={onChange} className={className}>
      <TabList aria-label={label} className="ui-tabs">
        {items.map((item) => (
          <Tab key={item.id} disabled={item.disabled} className="ui-tab">
            {item.label}
          </Tab>
        ))}
      </TabList>
      <TabPanels className="mt-5">
        {items.map((item) => (
          <TabPanel key={item.id} className="ui-tab-panel">
            {item.content}
          </TabPanel>
        ))}
      </TabPanels>
    </TabGroup>
  );
}
