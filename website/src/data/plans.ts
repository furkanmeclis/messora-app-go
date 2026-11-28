// Plans and prices come from the app's public API (GET /api/v1/plans), loaded
// once at build time; see src/lib/plans.ts and README "Plans and prices".
// plans-fallback.json is the last resort when the build cannot reach the API
// (same shape as the API response; also usable as a local fixture).
import fallback from './plans-fallback.json';
export const fallbackPlans = fallback;
