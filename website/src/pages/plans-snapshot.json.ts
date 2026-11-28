// The plan data this build was rendered with (read by server.mjs).
import { planData, P } from '../lib/plans';

export const GET = () => new Response(JSON.stringify({ ...P.toApiShape(planData.plans, planData.updatedAt), source: planData.source }), {
  headers: { 'Content-Type': 'application/json' },
});
