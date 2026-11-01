# Uncertain node job operations

Administrators can inspect acknowledged jobs that expired without a confirmed result:

`GET /api/admin/nodes/:nodeId/uncertain-jobs`

The endpoint requires an administrator session and returns at most 100 matching jobs for the selected node. It includes the operation type, server ID, attempt count, failure codes, and timestamps. It deliberately omits bearer credentials, lease tokens, nonces, and operation payloads.

A job appears only when all of these conditions hold:

- The job is `DEAD`.
- Its last error is `ACKNOWLEDGED_LEASE_EXPIRED`.
- Its operation is `FAILED` with `AGENT_JOB_LEASE_LOST`.

## Required operator handling

An acknowledged operation may have started and produced side effects before the agent or network failed. Therefore:

1. Identify the affected server using the returned server ID.
2. Inspect the server's actual state through trusted node access.
3. Compare observed state with the intended operation.
4. Record the investigation and decide the appropriate manual remediation.
5. Do not reset the job to pending or dispatch it again merely because its result is missing.

The endpoint is read-only. It does not retry jobs, change operation status, or infer whether a side effect happened. Automated recovery remains fail-closed.
