import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";

process.env.NEXT_PUBLIC_KEYCLOAK_CLIENT_ID = "kc-web";
process.env.NEXT_PUBLIC_KEYCLOAK_ISSUER_URL = "http://localhost:8081/realms/test";

const mockSearchParams = vi.hoisted(() => ({ value: "" }));

function createWrapper() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={qc}>{children}</QueryClientProvider>
  );
}

const { mockGetMe } = vi.hoisted(() => ({
  mockGetMe: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  usePathname: () => "/login",
  useSearchParams: () => new URLSearchParams(mockSearchParams.value),
}));

vi.mock("@multica/core/auth", () => {
  const authState = {
    sendCode: vi.fn(),
    verifyCode: vi.fn(),
    user: null,
    isLoading: false,
  };
  const useAuthStore = Object.assign(
    (selector: (s: typeof authState) => unknown) => selector(authState),
    { getState: () => authState },
  );
  return { useAuthStore };
});

vi.mock("@/features/auth/auth-cookie", () => ({
  setLoggedInCookie: vi.fn(),
}));

vi.mock("@multica/core/workspace", () => {
  const wsState = {
    workspace: null,
    hydrateWorkspace: vi.fn(),
  };
  const useWorkspaceStore = Object.assign(
    (selector: (s: typeof wsState) => unknown) => selector(wsState),
    { getState: () => wsState },
  );
  return { useWorkspaceStore };
});

vi.mock("@multica/core/api", () => ({
  api: {
    getMe: mockGetMe,
    issueCliToken: vi.fn(),
    listWorkspaces: vi.fn(),
    setToken: vi.fn(),
    verifyCode: vi.fn(),
  },
}));

import LoginPage from "./page";

describe("Web LoginPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockSearchParams.value = "";
    mockGetMe.mockRejectedValue(new Error("unauthorized"));
    Object.defineProperty(window, "location", {
      writable: true,
      value: { href: "http://localhost:3000/login" },
    });
  });

  it("renders Keycloak SSO button and hides email form", () => {
    render(<LoginPage />, { wrapper: createWrapper() });

    expect(screen.getByText("Sign in to Multica")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Continue with Keycloak" })).toBeInTheDocument();
    expect(screen.queryByLabelText("Email")).not.toBeInTheDocument();
  });

  it("includes desktop + cli callback state in SSO redirect", async () => {
    const cliCallback = "http://localhost:9876/callback";
    mockSearchParams.value = `platform=desktop&cli_callback=${encodeURIComponent(cliCallback)}&cli_state=abc123`;
    const user = userEvent.setup();

    render(<LoginPage />, { wrapper: createWrapper() });
    await user.click(screen.getByRole("button", { name: "Continue with Keycloak" }));

    await waitFor(() => {
      expect(window.location.href).toContain("/protocol/openid-connect/auth?");
      expect(window.location.href).toContain("client_id=kc-web");
      expect(window.location.href).toContain("response_type=code");
    });

    const redirectURL = new URL(window.location.href);
    const encodedState = redirectURL.searchParams.get("state");
    expect(encodedState).toBeTruthy();
    const decodedState = JSON.parse(atob(encodedState || ""));
    expect(decodedState).toMatchObject({
      platform: "desktop",
      cli_callback: cliCallback,
      cli_state: "abc123",
    });
  });
});
