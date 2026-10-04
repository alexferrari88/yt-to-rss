# Use secret URLs for podcast read access

Use long, unguessable HTTPS URLs for feed and audio, as accepted by the operator, rather than an interactive login or player-specific authentication integration. Possession grants read access only within the deployment's permitted network and grants no permission to submit or manage episodes. This keeps the read-access mechanism usable by ordinary RSS clients, at the cost of shared links acting as credentials and URL rotation potentially requiring a subscription update. Changing this contract later affects saved feed and episode URLs, so keep write credentials separate from the outset.

ADR-0002 adds the operator's Tailscale-only network boundary. Secret URLs do not bypass that boundary or make the installation public.
