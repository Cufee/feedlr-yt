import { ArrowRightIcon } from "@heroicons/react/24/outline";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { Metadata } from "@/app/Metadata";
import { Page, PageLoading, RequestError } from "@/app/Page";
import { api } from "@/lib/api/client";
import { useSession } from "@/lib/api/queries";

export function LandingPage() {
  const { data } = useSession();
  return (
    <div className="landing-page">
      <Metadata
        title="YouTube and podcasts"
        description="Follow channels and shows, save items, and resume playback."
      />
      <section className="landing-hero">
        <h1>
          YouTube
          <br />
          <span>and podcasts</span>
        </h1>
        <p>Follow channels and shows, save items, and resume playback.</p>
        <Link
          to={data?.authenticated ? "/app" : "/login"}
          className="ui-button ui-button-primary ui-button-lg"
        >
          {data?.authenticated ? "Open your feed" : "Sign in"}
          <ArrowRightIcon />
        </Link>
      </section>
    </div>
  );
}
export function LegalPage({ document }: { document: string }) {
  const valid = document === "privacy-policy" || document === "terms-of-service";
  const { data, isPending, error, refetch } = useQuery({
    queryKey: ["feedlr-legal", document],
    queryFn: ({ signal }) => api.get<{ html: string }>(`/legal/${document}`, signal),
    enabled: valid,
    staleTime: 3_600_000,
  });
  if (!valid) {
    return <NotFoundPage />;
  }
  return (
    <Page title={document === "privacy-policy" ? "Privacy Policy" : "Terms of Service"}>
      {isPending ? (
        <PageLoading />
      ) : error ? (
        <RequestError error={error} retry={() => void refetch()} />
      ) : (
        // biome-ignore lint/security/noDangerouslySetInnerHtml: The allowlisted legal endpoint sanitizes its HTML with the server's UGC policy.
        <article className="panel legal-prose" dangerouslySetInnerHTML={{ __html: data.html }} />
      )}
    </Page>
  );
}
export function NotFoundPage() {
  return (
    <Page title="Page not found">
      <Link to="/app" className="ui-link">
        Go to your feed
      </Link>
    </Page>
  );
}
export function ErrorPage({ limited = false }: { limited?: boolean }) {
  return (
    <Page title={limited ? "Too many requests" : "Something went wrong"}>
      <p className="muted">
        {limited ? "Wait a moment, then try again." : "Try again, or return to your feed."}
      </p>
      <Link to="/app" className="ui-link">
        Back to your feed
      </Link>
    </Page>
  );
}
