"use client";

import { Suspense, useEffect, useState } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import { useQueryClient } from "@tanstack/react-query";
import { useAuthStore } from "@multica/core/auth";
import { useWorkspaceStore } from "@multica/core/workspace";
import { workspaceKeys } from "@multica/core/workspace/queries";
import { api } from "@multica/core/api";
import { validateCliCallback } from "@multica/views/auth";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from "@multica/ui/components/ui/card";
import { Button } from "@multica/ui/components/ui/button";
import { Loader2 } from "lucide-react";

type AuthCallbackState = {
  platform?: string;
  cli_callback?: string;
  cli_state?: string;
};

function parseAuthState(raw: string | null): AuthCallbackState {
  if (!raw) return {};
  if (raw === "platform:desktop") {
    return { platform: "desktop" };
  }
  try {
    const decoded = atob(raw);
    const parsed = JSON.parse(decoded);
    return typeof parsed === "object" && parsed ? parsed : {};
  } catch {
    return {};
  }
}

function CallbackContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const qc = useQueryClient();
  const loginWithKeycloak = useAuthStore((s) => s.loginWithKeycloak);
  const hydrateWorkspace = useWorkspaceStore((s) => s.hydrateWorkspace);
  const [error, setError] = useState("");
  const [desktopToken, setDesktopToken] = useState<string | null>(null);

  useEffect(() => {
    const code = searchParams.get("code");
    if (!code) {
      setError("Missing authorization code");
      return;
    }

    const errorParam = searchParams.get("error");
    if (errorParam) {
      setError(errorParam === "access_denied" ? "Access denied" : errorParam);
      return;
    }

    const callbackState = parseAuthState(searchParams.get("state"));
    const isDesktop = callbackState.platform === "desktop";
    const cliCallback = callbackState.cli_callback;
    const cliState = callbackState.cli_state ?? "";

    const redirectUri = `${window.location.origin}/auth/callback`;

    if (isDesktop) {
      // Desktop flow: exchange code for token, then redirect via deep link
      api
        .keycloakLogin(code, redirectUri)
        .then(({ token }) => {
          setDesktopToken(token);
          window.location.href = `multica://auth/callback?token=${encodeURIComponent(token)}`;
        })
        .catch((err) => {
          setError(err instanceof Error ? err.message : "Login failed");
        });
    } else {
      // Normal web flow
      loginWithKeycloak(code, redirectUri)
        .then(async () => {
          if (cliCallback && validateCliCallback(cliCallback)) {
            const { token } = await api.issueCliToken();
            const separator = cliCallback.includes("?") ? "&" : "?";
            window.location.href = `${cliCallback}${separator}token=${encodeURIComponent(token)}&state=${encodeURIComponent(cliState)}`;
            return;
          }
          const wsList = await api.listWorkspaces();
          qc.setQueryData(workspaceKeys.list(), wsList);
          const lastWsId = localStorage.getItem("multica_workspace_id");
          const ws = await hydrateWorkspace(wsList, lastWsId);
          router.push(ws ? "/issues" : "/onboarding");
        })
        .catch((err) => {
          setError(err instanceof Error ? err.message : "Login failed");
        });
    }
  }, [searchParams, loginWithKeycloak, hydrateWorkspace, router, qc]);

  if (desktopToken) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <Card className="w-full max-w-sm">
          <CardHeader className="text-center">
            <CardTitle className="text-2xl">Opening Multica</CardTitle>
            <CardDescription>
              You should see a prompt to open the Multica desktop app. If
              nothing happens, click the button below.
            </CardDescription>
          </CardHeader>
          <CardContent className="flex justify-center">
            <Button
              variant="outline"
              onClick={() => {
                window.location.href = `multica://auth/callback?token=${encodeURIComponent(desktopToken)}`;
              }}
            >
              Open Multica Desktop
            </Button>
          </CardContent>
        </Card>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <Card className="w-full max-w-sm">
          <CardHeader className="text-center">
            <CardTitle className="text-2xl">Login Failed</CardTitle>
            <CardDescription>{error}</CardDescription>
          </CardHeader>
          <CardContent className="flex justify-center">
            <a href="/login" className="text-primary underline-offset-4 hover:underline">
              Back to login
            </a>
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="flex min-h-screen items-center justify-center">
      <Card className="w-full max-w-sm">
        <CardHeader className="text-center">
          <CardTitle className="text-2xl">Signing in...</CardTitle>
          <CardDescription>Please wait while we complete your login</CardDescription>
        </CardHeader>
        <CardContent className="flex justify-center">
          <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
        </CardContent>
      </Card>
    </div>
  );
}

export default function CallbackPage() {
  return (
    <Suspense fallback={null}>
      <CallbackContent />
    </Suspense>
  );
}
