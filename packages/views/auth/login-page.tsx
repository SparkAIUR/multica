"use client";

import { useState, useEffect, useCallback, useRef, type ReactNode } from "react";
import { useQueryClient } from "@tanstack/react-query";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
  CardFooter,
} from "@multica/ui/components/ui/card";
import { Input } from "@multica/ui/components/ui/input";
import { Button } from "@multica/ui/components/ui/button";
import { Label } from "@multica/ui/components/ui/label";
import {
  InputOTP,
  InputOTPGroup,
  InputOTPSlot,
} from "@multica/ui/components/ui/input-otp";
import { useAuthStore } from "@multica/core/auth";
import { useWorkspaceStore } from "@multica/core/workspace";
import { workspaceKeys } from "@multica/core/workspace/queries";
import { api } from "@multica/core/api";
import type { User } from "@multica/core/types";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface SSOAuthConfig {
  authorizeUrl: string;
  clientId: string;
  redirectUri: string;
  /** OAuth scope for the provider. */
  scope?: string;
  /** Opaque state passed through OAuth (e.g. platform or CLI metadata). */
  state?: string;
  /** Button label override. */
  label?: string;
}

interface CliCallbackConfig {
  /** Validated localhost callback URL */
  url: string;
  /** Opaque state to pass back to CLI */
  state: string;
}

interface LoginPageProps {
  /** Logo element rendered above the title */
  logo?: ReactNode;
  /** Called after successful login + workspace hydration */
  onSuccess: () => void;
  /** SSO/OIDC config. Omit to disable federated login button. */
  sso?: SSOAuthConfig;
  /** Whether email + code login should be rendered. Defaults to true. */
  emailAuthEnabled?: boolean;
  /** CLI callback config for authorizing CLI tools. */
  cliCallback?: CliCallbackConfig;
  /** Preferred workspace ID to restore after login. */
  lastWorkspaceId?: string | null;
  /** Called after a token is obtained (e.g. to set cookies). */
  onTokenObtained?: () => void;
  /** Override SSO login handler (e.g. desktop opens browser externally). */
  onSSOLogin?: () => void;
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function redirectToCliCallback(url: string, token: string, state: string) {
  const separator = url.includes("?") ? "&" : "?";
  window.location.href = `${url}${separator}token=${encodeURIComponent(token)}&state=${encodeURIComponent(state)}`;
}

/** Validate that a CLI callback URL points to localhost over HTTP. */
export function validateCliCallback(cliCallback: string): boolean {
  try {
    const cbUrl = new URL(cliCallback);
    if (cbUrl.protocol !== "http:") return false;
    if (cbUrl.hostname !== "localhost" && cbUrl.hostname !== "127.0.0.1")
      return false;
    return true;
  } catch {
    return false;
  }
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function LoginPage({
  logo,
  onSuccess,
  sso,
  emailAuthEnabled = true,
  cliCallback,
  lastWorkspaceId,
  onTokenObtained,
  onSSOLogin,
}: LoginPageProps) {
  const qc = useQueryClient();
  const [step, setStep] = useState<"email" | "code" | "cli_confirm">("email");
  const [email, setEmail] = useState("");
  const [code, setCode] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [cooldown, setCooldown] = useState(0);
  const [existingUser, setExistingUser] = useState<User | null>(null);
  // Tracks how the existing session was detected so handleCliAuthorize
  // uses the matching token source (cookie → issueCliToken, localStorage → direct).
  const authSourceRef = useRef<"cookie" | "localStorage">("cookie");

  // Check for existing session when CLI callback is present.
  // Prioritises cookie auth (= current browser session) to avoid authorising
  // the CLI with a stale or mismatched localStorage token.
  useEffect(() => {
    if (!cliCallback) return;

    // Ensure no stale bearer token interferes — we want to test the cookie first.
    api.setToken(null);

    api
      .getMe()
      .then((user) => {
        authSourceRef.current = "cookie";
        setExistingUser(user);
        setStep("cli_confirm");
      })
      .catch(() => {
        // Cookie auth failed — fall back to localStorage token
        const token = localStorage.getItem("multica_token");
        if (!token) return;

        api.setToken(token);
        api
          .getMe()
          .then((user) => {
            authSourceRef.current = "localStorage";
            setExistingUser(user);
            setStep("cli_confirm");
          })
          .catch(() => {
            api.setToken(null);
            localStorage.removeItem("multica_token");
          });
      });
  }, [cliCallback]);

  // Cooldown timer for resend
  useEffect(() => {
    if (cooldown <= 0) return;
    const timer = setTimeout(() => setCooldown((c) => c - 1), 1000);
    return () => clearTimeout(timer);
  }, [cooldown]);

  const handleSendCode = useCallback(
    async (e?: React.FormEvent) => {
      e?.preventDefault();
      if (!email) {
        setError("Email is required");
        return;
      }
      setLoading(true);
      setError("");
      try {
        await useAuthStore.getState().sendCode(email);
        setStep("code");
        setCode("");
        setCooldown(60);
      } catch (err) {
        setError(
          err instanceof Error
            ? err.message
            : "Failed to send code. Make sure the server is running.",
        );
      } finally {
        setLoading(false);
      }
    },
    [email],
  );

  const handleVerify = useCallback(
    async (value: string) => {
      if (value.length !== 6) return;
      setLoading(true);
      setError("");
      try {
        if (cliCallback) {
          // CLI path: get token directly for the redirect URL
          const { token } = await api.verifyCode(email, value);
          localStorage.setItem("multica_token", token);
          api.setToken(token);
          onTokenObtained?.();
          redirectToCliCallback(cliCallback.url, token, cliCallback.state);
          return;
        }

        // Normal path
        await useAuthStore.getState().verifyCode(email, value);
        const wsList = await api.listWorkspaces();
        qc.setQueryData(workspaceKeys.list(), wsList);
        useWorkspaceStore.getState().hydrateWorkspace(wsList, lastWorkspaceId);
        onTokenObtained?.();
        onSuccess();
      } catch (err) {
        setError(
          err instanceof Error ? err.message : "Invalid or expired code",
        );
        setCode("");
        setLoading(false);
      }
    },
    [email, onSuccess, cliCallback, lastWorkspaceId, onTokenObtained, qc],
  );

  const handleResend = async () => {
    if (cooldown > 0) return;
    setError("");
    try {
      await useAuthStore.getState().sendCode(email);
      setCooldown(60);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Failed to resend code",
      );
    }
  };

  const handleCliAuthorize = async () => {
    if (!cliCallback) return;
    setLoading(true);

    try {
      let token: string;

      if (authSourceRef.current === "localStorage") {
        // Session was detected via localStorage — reuse that token directly.
        const stored = localStorage.getItem("multica_token");
        if (!stored) throw new Error("token missing");
        token = stored;
      } else {
        // Session was detected via cookie — obtain a bearer token from the server.
        const res = await api.issueCliToken();
        token = res.token;
      }

      onTokenObtained?.();
      redirectToCliCallback(cliCallback.url, token, cliCallback.state);
    } catch {
      setError("Failed to authorize CLI. Please log in again.");
      setExistingUser(null);
      setStep("email");
      setLoading(false);
    }
  };

  const handleSSOLogin = () => {
    if (onSSOLogin) {
      onSSOLogin();
      return;
    }
    if (!sso) return;
    const params = new URLSearchParams({
      client_id: sso.clientId,
      redirect_uri: sso.redirectUri,
      response_type: "code",
      scope: sso.scope ?? "openid email profile",
    });
    if (sso.state) params.set("state", sso.state);
    window.location.href = `${sso.authorizeUrl}?${params}`;
  };

  // -------------------------------------------------------------------------
  // CLI confirm step
  // -------------------------------------------------------------------------

  if (step === "cli_confirm" && existingUser) {
    return (
      <div className="flex min-h-svh items-center justify-center">
        <Card className="w-full max-w-sm">
          <CardHeader className="text-center">
            {logo && <div className="mx-auto mb-4">{logo}</div>}
            <CardTitle className="text-2xl">Authorize CLI</CardTitle>
            <CardDescription>
              Allow the CLI to access Multica as{" "}
              <span className="font-medium text-foreground">
                {existingUser.email}
              </span>
              ?
            </CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col gap-3">
            <Button
              onClick={handleCliAuthorize}
              disabled={loading}
              className="w-full"
              size="lg"
            >
              {loading ? "Authorizing..." : "Authorize"}
            </Button>
            <Button
              variant="ghost"
              className="w-full"
              onClick={() => {
                setExistingUser(null);
                setStep("email");
              }}
            >
              Use a different account
            </Button>
          </CardContent>
        </Card>
      </div>
    );
  }

  // -------------------------------------------------------------------------
  // Code verification step
  // -------------------------------------------------------------------------

  if (step === "code") {
    return (
      <div className="flex min-h-svh items-center justify-center">
        <Card className="w-full max-w-sm">
          <CardHeader className="text-center">
            {logo && <div className="mx-auto mb-4">{logo}</div>}
            <CardTitle className="text-2xl">Check your email</CardTitle>
            <CardDescription>
              We sent a verification code to{" "}
              <span className="font-medium text-foreground">{email}</span>
            </CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col items-center gap-4">
            <InputOTP
              maxLength={6}
              value={code}
              onChange={(value) => {
                setCode(value);
                if (value.length === 6) handleVerify(value);
              }}
              disabled={loading}
            >
              <InputOTPGroup>
                <InputOTPSlot index={0} />
                <InputOTPSlot index={1} />
                <InputOTPSlot index={2} />
                <InputOTPSlot index={3} />
                <InputOTPSlot index={4} />
                <InputOTPSlot index={5} />
              </InputOTPGroup>
            </InputOTP>
            {error && (
              <p className="text-sm text-destructive">{error}</p>
            )}
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <button
                type="button"
                onClick={handleResend}
                disabled={cooldown > 0}
                className="text-primary underline-offset-4 hover:underline disabled:text-muted-foreground disabled:no-underline disabled:cursor-not-allowed"
              >
                {cooldown > 0 ? `Resend in ${cooldown}s` : "Resend code"}
              </button>
            </div>
          </CardContent>
          <CardFooter>
            <Button
              type="button"
              variant="ghost"
              className="w-full"
              onClick={() => {
                setStep("email");
                setCode("");
                setError("");
              }}
            >
              Back
            </Button>
          </CardFooter>
        </Card>
      </div>
    );
  }

  // -------------------------------------------------------------------------
  // Email step
  // -------------------------------------------------------------------------

  return (
    <div className="flex min-h-svh items-center justify-center">
      <Card className="w-full max-w-sm">
        <CardHeader className="text-center">
          {logo && <div className="mx-auto mb-4">{logo}</div>}
          <CardTitle className="text-2xl">Sign in to Multica</CardTitle>
          <CardDescription>
            {emailAuthEnabled
              ? "Enter your email to get a login code"
              : "Continue with your SSO provider"}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {emailAuthEnabled ? (
            <form id="login-form" onSubmit={handleSendCode} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="login-email">Email</Label>
                <Input
                  id="login-email"
                  type="email"
                  placeholder="you@example.com"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  autoFocus
                  required
                />
              </div>
              {error && (
                <p className="text-sm text-destructive">{error}</p>
              )}
            </form>
          ) : (
            error && <p className="text-sm text-destructive">{error}</p>
          )}
        </CardContent>
        <CardFooter className="flex flex-col gap-3">
          {emailAuthEnabled && (
            <Button
              type="submit"
              form="login-form"
              className="w-full"
              size="lg"
              disabled={!email || loading}
            >
              {loading ? "Sending code..." : "Continue"}
            </Button>
          )}
          {(sso || onSSOLogin) && (
            <>
              {emailAuthEnabled && (
                <div className="relative w-full">
                  <div className="absolute inset-0 flex items-center">
                    <span className="w-full border-t" />
                  </div>
                  <div className="relative flex justify-center text-xs uppercase">
                    <span className="bg-card px-2 text-muted-foreground">or</span>
                  </div>
                </div>
              )}
              <Button
                type="button"
                variant="outline"
                className="w-full"
                size="lg"
                onClick={handleSSOLogin}
                disabled={loading}
              >
                {sso?.label ?? "Continue with SSO"}
              </Button>
            </>
          )}
        </CardFooter>
      </Card>
    </div>
  );
}
