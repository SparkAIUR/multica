import { useEffect, useState } from "react";

const DEFAULT_WORKSPACE_URL_PREFIX = "multica.ai/";

export function useWorkspaceUrlPrefix() {
  const [workspaceUrlPrefix, setWorkspaceUrlPrefix] = useState(
    DEFAULT_WORKSPACE_URL_PREFIX,
  );

  useEffect(() => {
    const host = window.location.host;
    if (host) {
      setWorkspaceUrlPrefix(`${host}/`);
    }
  }, []);

  return workspaceUrlPrefix;
}
