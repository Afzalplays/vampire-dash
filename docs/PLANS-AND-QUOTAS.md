# Plans and quotas

Plans and entitlements must be enforced by the API and node runtime. UI values are informational only. A server create request must check active entitlement, slot count, RAM, disk, CPU, location capacity and port availability atomically before provisioning.

The initial proposed free tier is 4 GB RAM, 10 GB storage, 200% CPU and one server slot, subject to operator configuration and actual node capacity. Do not advertise a region until a healthy node is registered there. Paid 24/7 service must be an explicit entitlement. Expired accounts may be suspended according to published policy; customer data must not be silently deleted.
