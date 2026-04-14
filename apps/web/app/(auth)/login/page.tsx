"use client";

import { Suspense, useEffect } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import { useAuthStore } from "@multica/core/auth";
import { useWorkspaceStore } from "@multica/core/workspace";
import { setLoggedInCookie } from "@/features/auth/auth-cookie";
import { LoginPage, validateCliCallback } from "@multica/views/auth";

function encodeAuthState(payload: Record<string, string>): string {
  return btoa(JSON.stringify(payload));
}

function resolveKeycloakConfig() {
  const keycloakClientId = process.env.NEXT_PUBLIC_KEYCLOAK_CLIENT_ID;
  const keycloakIssuerURL =
    process.env.NEXT_PUBLIC_KEYCLOAK_ISSUER_URL?.replace(/\/$/, "");
  const keycloakAuthorizeURL =
    process.env.NEXT_PUBLIC_KEYCLOAK_AUTH_URL ||
    (keycloakIssuerURL
      ? `${keycloakIssuerURL}/protocol/openid-connect/auth`
      : undefined);
  return { keycloakClientId, keycloakAuthorizeURL };
}

function LoginPageContent() {
  const router = useRouter();
  const user = useAuthStore((s) => s.user);
  const isLoading = useAuthStore((s) => s.isLoading);
  const searchParams = useSearchParams();
  const { keycloakClientId, keycloakAuthorizeURL } = resolveKeycloakConfig();

  const cliCallbackRaw = searchParams.get("cli_callback");
  const cliState = searchParams.get("cli_state") || "";
  const platform = searchParams.get("platform");
  const nextUrl = searchParams.get("next") || "/issues";
  const cliCallback =
    cliCallbackRaw && validateCliCallback(cliCallbackRaw)
      ? { url: cliCallbackRaw, state: cliState }
      : undefined;

  const statePayload: Record<string, string> = {};
  if (platform === "desktop") statePayload.platform = "desktop";
  if (cliCallback) {
    statePayload.cli_callback = cliCallback.url;
    statePayload.cli_state = cliCallback.state;
  }
  const authState =
    Object.keys(statePayload).length > 0
      ? encodeAuthState(statePayload)
      : undefined;

  // Already authenticated — redirect to dashboard (skip if CLI callback)
  useEffect(() => {
    if (!isLoading && user && !cliCallbackRaw) {
      router.replace(nextUrl);
    }
  }, [isLoading, user, router, nextUrl, cliCallbackRaw]);

  const lastWorkspaceId =
    typeof window !== "undefined"
      ? localStorage.getItem("multica_workspace_id")
      : null;

  const handleSuccess = () => {
    const ws = useWorkspaceStore.getState().workspace;
    router.push(ws ? nextUrl : "/onboarding");
  };

  return (
    <LoginPage
      onSuccess={handleSuccess}
      sso={
        keycloakClientId && keycloakAuthorizeURL
          ? {
              authorizeUrl: keycloakAuthorizeURL,
              clientId: keycloakClientId,
              redirectUri: `${window.location.origin}/auth/callback`,
              state: authState,
              scope: "openid email profile",
              label: "Continue with Keycloak",
            }
          : undefined
      }
      emailAuthEnabled={false}
      cliCallback={cliCallback}
      lastWorkspaceId={lastWorkspaceId}
      onTokenObtained={setLoggedInCookie}
    />
  );
}

export default function Page() {
  return (
    <Suspense fallback={null}>
      <LoginPageContent />
    </Suspense>
  );
}
