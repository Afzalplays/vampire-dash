export function serverStatusAfterAgentJobFailure(operationType: string, jobStatus: string): "UNKNOWN" | "ERROR" | null {
  if (operationType === "PROVISION_SERVER") return jobStatus === "ACKNOWLEDGED" ? "UNKNOWN" : "ERROR";
  if (jobStatus === "ACKNOWLEDGED") return "UNKNOWN";
  return null;
}
