import React, { useState } from 'react';
import { Server, Cpu, MemoryStick as Memory, HardDrive, CheckCircle, XCircle, Loader2, ChevronDown, ChevronUp } from 'lucide-react';
import { HostPreflightStatus } from '../../../types';
import { useConfig } from '../../../contexts/ConfigContext';

interface NodeMetric {
  cpu: number;
  memory: number;
  storage: {
    used: number;
    total: number;
  };
  dataPath: string;
}

interface PendingNode {
  name: string;
  type: 'application' | 'database';
  preflightStatus: HostPreflightStatus;
  progress: number;
  currentMessage: string;
  logs: string[];
  error?: string;
}

interface NodeMetricsProps {
  nodes: {
    application: {
      [key: string]: NodeMetric;
    };
    database: {
      [key: string]: NodeMetric;
    };
  };
  pendingNodes?: PendingNode[];
  isMultiNode?: boolean;
}

const NodeMetrics: React.FC<NodeMetricsProps> = ({ nodes, pendingNodes = [], isMultiNode = true }) => {
  const { prototypeSettings } = useConfig();
  const themeColor = prototypeSettings.themeColor;

  const renderMetricBar = (value: number, warningThreshold = 70) => {
    const color = value >= warningThreshold ? 'rgb(249 115 22)' : themeColor;
    return (
      <div className="flex-1 bg-gray-200 rounded-full h-2 ml-2">
        <div
          className="h-2 rounded-full transition-colors"
          style={{ 
            width: `${value}%`,
            backgroundColor: color
          }}
        />
      </div>
    );
  };

  const formatStorage = (gb: number) => {
    return `${gb}GB`;
  };

  const getFailedChecks = (status: HostPreflightStatus) => {
    return Object.entries(status)
      .filter(([_, result]) => result && !result.success)
      .map(([key, result]) => ({
        key,
        label: key.replace(/([A-Z])/g, ' $1').replace(/^./, str => str.toUpperCase()),
        message: result?.message || ''
      }));
  };

  const renderPreflightStatus = (nodeId: string, status: HostPreflightStatus, progress: number, message: string, error?: string) => {
    const failedChecks = getFailedChecks(status);
    const hasFailures = failedChecks.length > 0;

    return (
      <div className="space-y-2 mt-4 border-t border-gray-200 pt-4">
        <div className="mb-4">
          <div className="w-full bg-gray-200 rounded-full h-2">
            <div
              className="h-2 rounded-full transition-all duration-300"
              style={{ 
                width: `${progress}%`,
                backgroundColor: error ? 'rgb(239 68 68)' : themeColor
              }}
            />
          </div>
          <p className="text-sm text-gray-500 mt-2">{message}</p>
        </div>

        {hasFailures && (
          <div className="space-y-3">
            {failedChecks.map(({ key, label, message }) => (
              <div key={key} className="flex items-start">
                <XCircle className="w-4 h-4 text-red-500 mt-0.5 mr-2 flex-shrink-0" />
                <div>
                  <h5 className="text-sm font-medium text-red-800">{label}</h5>
                  <p className="mt-1 text-sm text-red-700">{message}</p>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    );
  };

  const renderNode = (name: string, type: string, metrics: NodeMetric | null, pendingNode?: PendingNode) => (
    <div className="bg-white rounded-lg border border-gray-200 p-4">
      <div className="flex items-center mb-4">
        <Server className="w-5 h-5 mr-2" style={{ color: themeColor }} />
        <div>
          <h3 className="text-lg font-medium text-gray-900">{name}</h3>
          <p className="text-sm text-gray-500">{type}</p>
        </div>
        <div className="ml-auto">
          {metrics ? (
            <div className="w-2 h-2 rounded-full" style={{ backgroundColor: themeColor }} />
          ) : pendingNode?.error ? (
            <XCircle className="w-5 h-5 text-red-500" />
          ) : (
            <Loader2 className="w-5 h-5 animate-spin text-blue-500" />
          )}
        </div>
      </div>

      {metrics ? (
        <div className="space-y-4">
          <div className="flex items-center">
            <Cpu className="w-4 h-4 text-gray-400" />
            <span className="ml-2 w-16 text-sm text-gray-600">CPU</span>
            {renderMetricBar(metrics.cpu)}
            <span className="ml-2 text-sm text-gray-600">{metrics.cpu}%</span>
          </div>

          <div className="flex items-center">
            <Memory className="w-4 h-4 text-gray-400" />
            <span className="ml-2 w-16 text-sm text-gray-600">Memory</span>
            {renderMetricBar(metrics.memory)}
            <span className="ml-2 text-sm text-gray-600">{metrics.memory}%</span>
          </div>

          <div className="flex items-center">
            <HardDrive className="w-4 h-4 text-gray-400" />
            <span className="ml-2 w-16 text-sm text-gray-600">Storage</span>
            {renderMetricBar((metrics.storage.used / metrics.storage.total) * 100)}
            <span className="ml-2 text-sm text-gray-600">
              {formatStorage(metrics.storage.used)} / {formatStorage(metrics.storage.total)}
            </span>
          </div>

          <div className="text-sm text-gray-500 mt-2">
            {metrics.dataPath}
          </div>
        </div>
      ) : pendingNode && (
        renderPreflightStatus(
          name, 
          pendingNode.preflightStatus,
          pendingNode.progress,
          pendingNode.currentMessage,
          pendingNode.error
        )
      )}
    </div>
  );

  return (
    <div className="space-y-6">
      <h2 className="text-lg font-medium text-gray-900">{isMultiNode ? 'Hosts' : 'Host'}</h2>
      
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {Object.entries(nodes.application).map(([name, metrics]) => 
          renderNode(name, 'Application', metrics)
        )}
        {Object.entries(nodes.database).map(([name, metrics]) => 
          renderNode(name, 'Database', metrics)
        )}
        {pendingNodes.map(node => 
          renderNode(
            node.name, 
            node.type === 'application' ? 'Application' : 'Database',
            null,
            node
          )
        )}
      </div>
    </div>
  );
};

export default NodeMetrics;