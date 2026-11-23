'use client';

import '@xyflow/react/dist/style.css';
import './canvas.css';

import {
    Background,
    BackgroundVariant,
    ConnectionLineType,
    Controls,
    MiniMap,
    ReactFlow,
    type ReactFlowProps,
} from '@xyflow/react';
import { useT } from '@/i18n/client';
import type { FlowEdge, FlowNode } from './graph';
import { stepCategory } from './model';
import { edgeTypes, nodeTypes } from './nodes';

const MINIMAP_COLOURS: Record<string, string> = {
    trigger: 'var(--primary)',
    message: 'oklch(68.5% 0.169 237.323)',
    data: 'oklch(60.6% 0.25 292.717)',
    flow: 'oklch(76.9% 0.188 70.08)',
    integration: 'oklch(69.6% 0.17 162.48)',
};

function minimapColour(n: FlowNode): string {
    if (n.type === 'trigger') return MINIMAP_COLOURS.trigger!;
    return MINIMAP_COLOURS[stepCategory(n.data.step.type)]!;
}

/**
 * React Flow with the panel's look: dotted background, zoom controls and a
 * minimap, labelled in the user's language. Editor and run trace share it.
 */
export function FlowCanvas(
    props: ReactFlowProps<FlowNode, FlowEdge> & { children?: React.ReactNode },
) {
    const t = useT();
    const { children, ...rest } = props;
    return (
        <ReactFlow<FlowNode, FlowEdge>
            nodeTypes={nodeTypes}
            edgeTypes={edgeTypes}
            defaultEdgeOptions={{ type: 'flow' }}
            connectionLineType={ConnectionLineType.SmoothStep}
            fitView
            fitViewOptions={{ padding: 0.2, maxZoom: 1 }}
            minZoom={0.15}
            maxZoom={1.75}
            attributionPosition="top-right"
            ariaLabelConfig={{
                'controls.ariaLabel': t('automations.canvas.controls'),
                'controls.zoomIn.ariaLabel': t('automations.canvas.zoomIn'),
                'controls.zoomOut.ariaLabel': t('automations.canvas.zoomOut'),
                'controls.fitView.ariaLabel': t('automations.canvas.fitView'),
                'minimap.ariaLabel': t('automations.canvas.minimap'),
            }}
            {...rest}
            // The minimap only appears when the canvas is wide enough not to
            // cover the flow itself.
            className={`@container ${rest.className ?? ''}`}
        >
            <Background variant={BackgroundVariant.Dots} gap={22} size={1.6} />
            <Controls showInteractive={false} position="bottom-left" />
            <MiniMap
                position="bottom-right"
                pannable
                zoomable
                nodeColor={minimapColour}
                nodeBorderRadius={8}
                className="hidden @[60rem]:block"
            />
            {children}
        </ReactFlow>
    );
}
