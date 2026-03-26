let calls = 0;

onmessage = function(event) {
  calls += 1;
  const payload = event && event.data && typeof event.data === "object" ? event.data : {};
  const numericValue = Number(payload.value || 0);

  if (payload.mode === "postMessage") {
    postMessage({
      mode: "postMessage",
      value: numericValue + 1,
      calls: calls,
      worker_id: event && event.worker_id ? event.worker_id : "",
      script_path: event && event.script_path ? event.script_path : ""
    });
    return null;
  }

  return {
    mode: "request",
    doubled: numericValue * 2,
    calls: calls,
    worker_id: event && event.worker_id ? event.worker_id : "",
    script_path: event && event.script_path ? event.script_path : ""
  };
};
