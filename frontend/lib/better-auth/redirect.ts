// Only follow same-site paths, so ?redirect= can't send users to another domain
export function safeRedirect(target: string | null | undefined, fallback = "/"): string {
  return target && /^\/(?![/\\])/.test(target) ? target : fallback;
}

// Where a user lands after signing in when there is no ?redirect=
export function homeForRole(role: string | null | undefined): string {
  return role === "publisher" ? "/publisher" : "/";
}
