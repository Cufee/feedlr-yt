import { ChevronLeftIcon, ChevronRightIcon } from "@heroicons/react/24/outline";
import { Link } from "@tanstack/react-router";
import type { ReactNode } from "react";
import { Button, EmptyState, Skeleton } from "@/components/ui";
import { ApiError } from "@/lib/api/client";
import type { MediaSource } from "@/lib/api/types";
import { Metadata } from "./Metadata";

export function Page({
  eyebrow,
  title,
  description,
  action,
  image,
  avatar,
  showTitle = false,
  showDescription = true,
  children,
}: {
  eyebrow?: string;
  title: string;
  description?: string;
  action?: ReactNode;
  image?: string;
  avatar?: string;
  showTitle?: boolean;
  showDescription?: boolean;
  children: ReactNode;
}) {
  const visibleDescription = showDescription ? description?.trim() : undefined;
  return (
    <div className="page-stack">
      <Metadata title={title} description={description} image={image} />
      {(showTitle || eyebrow || avatar || visibleDescription || action) && (
        <header className="page-heading">
          {(showTitle || eyebrow || avatar || visibleDescription) && (
            <div className="page-heading-copy">
              {avatar && <img className="page-avatar" src={avatar} alt="" />}
              <div className="page-heading-text">
                {eyebrow && <p className="eyebrow">{eyebrow}</p>}
                {showTitle && <h1 className="page-title">{title}</h1>}
                {visibleDescription && <p className="page-description">{visibleDescription}</p>}
              </div>
            </div>
          )}
          {action && <div className="page-actions">{action}</div>}
        </header>
      )}
      {children}
    </div>
  );
}
export function Section({
  title,
  action,
  children,
}: {
  title: string;
  action?: ReactNode;
  children: ReactNode;
}) {
  return (
    <section className="section-stack">
      <header className="section-heading">
        <h2>{title}</h2>
        {action}
      </header>
      {children}
    </section>
  );
}
export function PageLoading() {
  return (
    <div className="page-stack" role="status" aria-label="Loading page">
      <Skeleton className="h-12 w-64" />
      <div className="media-grid">
        {["one", "two", "three", "four", "five", "six"].map((id) => (
          <Skeleton key={id} className="aspect-video" />
        ))}
      </div>
    </div>
  );
}
export function RequestError({ error, retry }: { error: Error; retry?: () => void }) {
  const expired = error instanceof ApiError && error.status === 401;
  return (
    <EmptyState
      title={expired ? "Sign in to continue" : "Couldn’t load this page"}
      description={error.message}
      action={
        expired ? (
          <Link to="/login" className="ui-link">
            Sign in
          </Link>
        ) : retry ? (
          <Button onClick={retry}>Try again</Button>
        ) : undefined
      }
    />
  );
}
export function SourceNavigation({
  source,
  to,
}: {
  source: MediaSource;
  to: "/app/recent" | "/app/subscriptions";
}) {
  const choices = [
    { value: "all", label: "Everything" },
    { value: "youtube", label: "YouTube" },
    { value: "podcasts", label: "Podcasts" },
  ] as const;
  return (
    <nav className="segmented-navigation" aria-label="Media sources">
      {choices.map((choice) => (
        <Link
          key={choice.value}
          to={to}
          search={{ source: choice.value }}
          resetScroll={false}
          aria-current={source === choice.value ? "page" : undefined}
          className={source === choice.value ? "is-active" : ""}
        >
          {choice.label}
        </Link>
      ))}
    </nav>
  );
}

export function Pagination({
  page,
  hasMore,
  to,
}: {
  page: number;
  hasMore: boolean;
  to: "/app/watch-later";
}) {
  if (page <= 1 && !hasMore) {
    return null;
  }
  return (
    <nav className="pagination" aria-label="Pages">
      {page > 1 ? (
        <Link className="ui-button" to={to} search={{ page: page - 1 }}>
          <ChevronLeftIcon />
          Previous
        </Link>
      ) : (
        <span />
      )}
      <span className="muted">Page {page}</span>
      {hasMore ? (
        <Link className="ui-button" to={to} search={{ page: page + 1 }}>
          Next
          <ChevronRightIcon />
        </Link>
      ) : (
        <span />
      )}
    </nav>
  );
}
