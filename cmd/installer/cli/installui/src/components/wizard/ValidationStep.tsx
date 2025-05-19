import React, { useState, useEffect } from 'react';
import Card from '../common/Card';
import Button from '../common/Button';
import { useConfig } from '../../contexts/ConfigContext';
import { ValidationStatus } from '../../types';
import { ChevronLeft, ChevronRight, CheckCircle, XCircle, AlertTriangle, Loader2 } from 'lucide-react';
import { validateEnvironment } from '../../utils/validation';

interface ValidationStepProps {
  onNext: () => void;
  onBack: () => void;
}

const ValidationStep: React.FC<ValidationStepProps> = ({ onNext, onBack }) => {
  const { config } = useConfig();
  const [validationStatus, setValidationStatus] = useState<ValidationStatus>({
    kubernetes: null,
    helm: null,
    storage: null,
    networking: null,
    permissions: null,
  });
  const [isValidating, setIsValidating] = useState(false);
  const [validationComplete, setValidationComplete] = useState(false);
  const [overallStatus, setOverallStatus] = useState<'success' | 'warning' | 'error' | null>(null);

  const startValidation = async () => {
    setIsValidating(true);
    setValidationComplete(false);
    setOverallStatus(null);

    try {
      const results = await validateEnvironment(config);
      setValidationStatus(results);

      // Determine overall status
      const hasErrors = Object.values(results).some((result) => result && !result.success);
      const hasWarnings = Object.values(results).some(
        (result) => result && result.success && result.message.includes('warning')
      );

      if (hasErrors) {
        setOverallStatus('error');
      } else if (hasWarnings) {
        setOverallStatus('warning');
      } else {
        setOverallStatus('success');
      }
    } catch (error) {
      console.error('Validation error:', error);
      setOverallStatus('error');
    } finally {
      setIsValidating(false);
      setValidationComplete(true);
    }
  };

  useEffect(() => {
    startValidation();
  }, []);

  const renderValidationItem = (title: string, status: 'pending' | 'running' | 'success' | 'warning' | 'error') => {
    let Icon;
    let statusColor;
    let statusText;

    switch (status) {
      case 'pending':
        Icon = AlertTriangle;
        statusColor = 'text-gray-400';
        statusText = 'Pending';
        break;
      case 'running':
        Icon = Loader2;
        statusColor = 'text-blue-500';
        statusText = 'Validating...';
        break;
      case 'success':
        Icon = CheckCircle;
        statusColor = 'text-green-500';
        statusText = 'Passed';
        break;
      case 'warning':
        Icon = AlertTriangle;
        statusColor = 'text-yellow-500';
        statusText = 'Warning';
        break;
      case 'error':
        Icon = XCircle;
        statusColor = 'text-red-500';
        statusText = 'Failed';
        break;
    }

    return (
      <div className="flex items-center space-x-4 py-3">
        <div className={`flex-shrink-0 ${statusColor}`}>
          <Icon className={`w-6 h-6 ${status === 'running' ? 'animate-spin' : ''}`} />
        </div>
        <div className="flex-grow">
          <h4 className="text-sm font-medium text-gray-900">{title}</h4>
        </div>
        <div className={`text-sm font-medium ${statusColor}`}>{statusText}</div>
      </div>
    );
  };

  const getValidationStatusType = (
    result: { success: boolean; message: string } | null,
    isRunning: boolean
  ): 'pending' | 'running' | 'success' | 'warning' | 'error' => {
    if (isRunning) return 'running';
    if (!result) return 'pending';
    if (!result.success) return 'error';
    if (result.message.toLowerCase().includes('warning')) return 'warning';
    return 'success';
  };

  return (
    <div className="space-y-6">
      <Card>
        <div className="mb-6">
          <h2 className="text-2xl font-bold text-gray-900">Environment Validation</h2>
          <p className="text-gray-600 mt-1">
            We'll check if your Kubernetes environment meets all the requirements for Gitea Enterprise.
          </p>
        </div>

        <div className="space-y-2 divide-y divide-gray-200">
          {renderValidationItem(
            'Kubernetes Availability',
            getValidationStatusType(validationStatus.kubernetes, isValidating)
          )}
          {renderValidationItem('Helm Installation', getValidationStatusType(validationStatus.helm, isValidating))}
          {renderValidationItem(
            'Storage Classes & PV Provisioning',
            getValidationStatusType(validationStatus.storage, isValidating)
          )}
          {renderValidationItem(
            'Networking & Ingress',
            getValidationStatusType(validationStatus.networking, isValidating)
          )}
          {renderValidationItem(
            'RBAC & Permissions',
            getValidationStatusType(validationStatus.permissions, isValidating)
          )}
        </div>

        {validationComplete && (
          <div
            className={`mt-6 p-4 rounded-md ${
              overallStatus === 'success'
                ? 'bg-green-50 text-green-800'
                : overallStatus === 'warning'
                ? 'bg-yellow-50 text-yellow-800'
                : 'bg-red-50 text-red-800'
            }`}
          >
            <div className="flex">
              <div className="flex-shrink-0">
                {overallStatus === 'success' ? (
                  <CheckCircle className="h-5 w-5 text-green-400" />
                ) : overallStatus === 'warning' ? (
                  <AlertTriangle className="h-5 w-5 text-yellow-400" />
                ) : (
                  <XCircle className="h-5 w-5 text-red-400" />
                )}
              </div>
              <div className="ml-3">
                <h3 className="text-sm font-medium">
                  {overallStatus === 'success'
                    ? 'All checks passed successfully!'
                    : overallStatus === 'warning'
                    ? 'Validation completed with warnings'
                    : 'Validation failed'}
                </h3>
                <div className="mt-2 text-sm">
                  {overallStatus === 'success' ? (
                    <p>Your environment is ready for Gitea Enterprise installation.</p>
                  ) : overallStatus === 'warning' ? (
                    <p>
                      Your environment may work but there are potential issues that could affect performance or
                      stability.
                    </p>
                  ) : (
                    <p>
                      There are critical issues that must be resolved before proceeding with the installation.
                    </p>
                  )}
                </div>
              </div>
            </div>
          </div>
        )}

        {!isValidating && (
          <div className="mt-6">
            <Button onClick={startValidation} variant="outline" size="sm">
              Revalidate Environment
            </Button>
          </div>
        )}
      </Card>

      <div className="flex justify-between">
        <Button variant="outline" onClick={onBack} icon={<ChevronLeft className="w-5 h-5" />}>
          Back
        </Button>
        <Button
          onClick={onNext}
          disabled={isValidating || overallStatus === 'error'}
          icon={<ChevronRight className="w-5 h-5" />}
        >
          Next: Install Gitea
        </Button>
      </div>
    </div>
  );
};

export default ValidationStep;