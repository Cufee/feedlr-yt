import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Badge, Switch } from "@/components/ui";
import { api } from "@/lib/api/client";
import { keys } from "@/lib/api/queries";
import type { Settings, SkipSettings as SkipSettingsData } from "@/lib/api/types";

type Provider = "sponsorBlock" | "podcastSegments";
export function SkipSettingsView({
  title,
  description,
  settings,
  pending = false,
  error,
  onChange,
}: {
  title: string;
  description: string;
  settings: SkipSettingsData;
  pending?: boolean;
  error?: string;
  onChange: (patch: { enabled?: boolean; categories?: string[] }) => void;
}) {
  return (
    <section className="panel settings-section">
      <header className="settings-section-header">
        <div>
          <div className="settings-title">
            <h2>{title}</h2>
            <Badge tone={settings.enabled ? "success" : "neutral"}>
              {settings.enabled ? "Enabled" : "Paused"}
            </Badge>
          </div>
          <p>{description}</p>
        </div>
      </header>
      <Switch
        label={`Enable ${title}`}
        checked={settings.enabled}
        disabled={pending}
        onChange={(enabled) => onChange({ enabled })}
      />
      <div className="settings-category-grid">
        {settings.availableCategories.map((category) => (
          <div className="settings-category" key={category.value}>
            <Switch
              label={category.name}
              description={category.description}
              checked={settings.categories.includes(category.value)}
              disabled={!settings.enabled || pending}
              onChange={(checked) =>
                onChange({
                  categories: checked
                    ? [...settings.categories, category.value]
                    : settings.categories.filter((value) => value !== category.value),
                })
              }
            />
          </div>
        ))}
      </div>
      {pending && (
        <p className="muted" role="status">
          Saving preferences…
        </p>
      )}
      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}
    </section>
  );
}
export function SkipSettings({
  provider,
  settings,
}: {
  provider: Provider;
  settings: SkipSettingsData;
}) {
  const client = useQueryClient();
  const mutation = useMutation({
    mutationKey: ["skip-settings", provider],
    mutationFn: (patch: { enabled?: boolean; categories?: string[] }) =>
      api.patch("/settings", { [provider]: patch }),
    onMutate: async (patch) => {
      await client.cancelQueries({ queryKey: keys.settings });
      const previous = client.getQueryData<Settings>(keys.settings)?.[provider];
      client.setQueryData<Settings>(keys.settings, (current) =>
        current ? { ...current, [provider]: { ...current[provider], ...patch } } : current,
      );
      return previous;
    },
    onError: (error, _patch, previous) => {
      if (previous) {
        client.setQueryData<Settings>(keys.settings, (current) =>
          current ? { ...current, [provider]: previous } : current,
        );
      }
      toast.error(error.message, { duration: 10_000 });
    },
    onSettled: () => client.invalidateQueries({ queryKey: keys.all }),
  });
  return (
    <SkipSettingsView
      title={provider === "sponsorBlock" ? "SponsorBlock" : "Smart skipping"}
      description={
        provider === "sponsorBlock"
          ? "Choose which community-marked YouTube segments to skip."
          : "Find optional moments in podcast transcripts and skip the categories you choose."
      }
      settings={settings}
      pending={mutation.isPending}
      error={mutation.error?.message}
      onChange={(patch) => mutation.mutate(patch)}
    />
  );
}
