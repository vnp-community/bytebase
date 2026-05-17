import { useMemo, useState } from "react";
import { useVueState } from "@/react/hooks/useVueState";
import { useEnvironmentV1Store } from "@/store";

/**
 * Data hook for InstanceFormBody.
 * Manages: form state, validation, engine-specific field visibility.
 */
export function useInstanceFormData(initialEngine?: string) {
  const envStore = useEnvironmentV1Store();
  const environments = useVueState(() => envStore.environmentList);

  const [engine, setEngine] = useState(initialEngine ?? "MYSQL");
  const [host, setHost] = useState("");
  const [port, setPort] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [database, setDatabase] = useState("");
  const [environment, setEnvironment] = useState("");
  const [title, setTitle] = useState("");
  const [sslEnabled, setSslEnabled] = useState(false);
  const [sshEnabled, setSshEnabled] = useState(false);

  const isValid = useMemo(() => {
    return (
      host.trim().length > 0 &&
      title.trim().length > 0 &&
      environment.length > 0
    );
  }, [host, title, environment]);

  return {
    engine,
    setEngine,
    host,
    setHost,
    port,
    setPort,
    username,
    setUsername,
    password,
    setPassword,
    database,
    setDatabase,
    environment,
    setEnvironment,
    title,
    setTitle,
    sslEnabled,
    setSslEnabled,
    sshEnabled,
    setSshEnabled,
    environments,
    isValid,
  };
}
