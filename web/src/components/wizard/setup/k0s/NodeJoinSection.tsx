import React from 'react';
import { Server, Copy, ClipboardCheck } from 'lucide-react';
import Button from '../../../common/Button';

interface NodeJoinSectionProps {
  selectedRole: 'application' | 'database';
  joinedNodes: { application: number; database: number };
  requiredNodes: { application: number; database: number };
  onRoleChange: (role: 'application' | 'database') => void;
  onCopyCommand: () => void;
  onStartNodeJoin: () => void;
  copied: boolean;
  themeColor: string;
  skipNodeValidation: boolean;
}

const NodeJoinSection: React.FC<NodeJoinSectionProps> = ({
  selectedRole,
  joinedNodes,
  requiredNodes,
  onRoleChange,
  onCopyCommand,
  onStartNodeJoin,
  copied,
  themeColor,
  skipNodeValidation,
}) => {
  const allNodesJoined = skipNodeValidation || (
    joinedNodes.application >= requiredNodes.application &&
    joinedNodes.database >= requiredNodes.database
  );

  return (
    <div className="mt-6 p-4 bg-gray-50 rounded-lg border border-gray-200">
      <h3 className="text-sm font-medium text-gray-900 mb-2">Join Additional Hosts</h3>
      <div className="flex items-center justify-between mb-4">
        <p className="text-sm text-gray-600">
          Hosts joined: {joinedNodes.application}/{requiredNodes.application} application, {joinedNodes.database}/{requiredNodes.database} database
        </p>
      </div>
      
      <div className="mb-4 space-x-4">
        <button
          onClick={() => onRoleChange('application')}
          disabled={joinedNodes.application >= requiredNodes.application}
          className="inline-flex items-center px-3 py-2 rounded-md transition-colors"
          style={{
            backgroundColor: selectedRole === 'application' ? themeColor : joinedNodes.application >= requiredNodes.application ? 'rgb(243 244 246)' : 'white',
            color: selectedRole === 'application' ? 'white' : joinedNodes.application >= requiredNodes.application ? 'rgb(156 163 175)' : 'rgb(55 65 81)',
            border: selectedRole === 'application' ? 'none' : '1px solid rgb(229 231 235)',
            cursor: joinedNodes.application >= requiredNodes.application ? 'not-allowed' : 'pointer',
          }}
        >
          <Server className="w-4 h-4 mr-2" />
          Application Host ({joinedNodes.application}/{requiredNodes.application})
        </button>
        <button
          onClick={() => onRoleChange('database')}
          disabled={joinedNodes.database >= requiredNodes.database}
          className="inline-flex items-center px-3 py-2 rounded-md transition-colors"
          style={{
            backgroundColor: selectedRole === 'database' ? themeColor : joinedNodes.database >= requiredNodes.database ? 'rgb(243 244 246)' : 'white',
            color: selectedRole === 'database' ? 'white' : joinedNodes.database >= requiredNodes.database ? 'rgb(156 163 175)' : 'rgb(55 65 81)',
            border: selectedRole === 'database' ? 'none' : '1px solid rgb(229 231 235)',
            cursor: joinedNodes.database >= requiredNodes.database ? 'not-allowed' : 'pointer',
          }}
        >
          <Server className="w-4 h-4 mr-2" />
          Database Host ({joinedNodes.database}/{requiredNodes.database})
        </button>
      </div>

      <div className="bg-gray-900 rounded-md p-4 flex items-center justify-between">
        <code className="text-gray-200 text-sm font-mono">
          sudo ./gitea-mastodon join 10.128.0.45:30000 {selectedRole === 'application' ? 'EaKuL6cNeIlzMci3JdDU9Oi4' : 'Xm9pK4vRtY2wQn8sLj5uH7fB'}
        </code>
        <div className="flex space-x-2">
          <Button
            variant="outline"
            size="sm"
            onClick={onCopyCommand}
            icon={copied ? <ClipboardCheck className="w-4 h-4" /> : <Copy className="w-4 h-4" />}
          >
            {copied ? 'Copied!' : 'Copy'}
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={onStartNodeJoin}
            disabled={joinedNodes[selectedRole] >= requiredNodes[selectedRole]}
          >
            Start Host Join
          </Button>
        </div>
      </div>

      {allNodesJoined && (
        <div className="mt-4 p-4 bg-green-50 rounded-md border border-green-200">
          <div className="flex items-start">
            <div className="ml-3">
              <h4 className="text-sm font-medium text-green-800">
                {skipNodeValidation ? 'Host validation skipped' : 'All required hosts have joined'}
              </h4>
              <p className="mt-1 text-sm text-green-600">
                You can now proceed with the installation
              </p>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default NodeJoinSection;