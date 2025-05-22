import React, { useState } from 'react';
import Card from '../common/Card';
import Button from '../common/Button';
import Input from '../common/Input';
import { GiteaLogo } from '../common/Logo';
import { ChevronRight, Lock, AlertTriangle } from 'lucide-react';
import { useWizardMode } from '../../contexts/WizardModeContext';
import { useConfig } from '../../contexts/ConfigContext';

interface WelcomeStepProps {
  onNext: () => void;
}

const WelcomeStep: React.FC<WelcomeStepProps> = ({ onNext }) => {
  const { text } = useWizardMode();
  const { prototypeSettings } = useConfig();
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [showPasswordInput, setShowPasswordInput] = useState(!prototypeSettings.useSelfSignedCert);

  const handlePasswordChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setPassword(e.target.value);
    setError('');
  };

  const handleSubmit = async () => {
    if (!showPasswordInput) {
      setShowPasswordInput(true);
      return;
    }

    if (!password.trim()) {
      setError('Password is required');
      return;
    }

    setIsLoading(true);
    setError('');
    
    try {
      const response = await fetch('/api/auth/login', {
        method: 'POST',
        body: JSON.stringify({ password }),
        headers: {
          'Content-Type': 'application/json'
        }
      });
      
      if (response.ok) {
        // Store the password in localStorage
        const data = await response.json();
        localStorage.setItem('auth', data.sessionToken);
        onNext();
      } else {
        setError('Invalid password');
      }
    } catch (err) {
      setError('Authentication failed. Please try again.');
      console.error('Login error:', err);
    } finally {
      setIsLoading(false);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && showPasswordInput) {
      handleSubmit();
    }
  };

  return (
    <div className="space-y-6">
      <Card>
        <div className="flex flex-col items-center text-center py-12">
          <GiteaLogo className="h-20 w-20 mb-6" />
          <h2 className="text-3xl font-bold text-gray-900">{text.welcomeTitle}</h2>
          <p className="text-xl text-gray-600 max-w-2xl mb-8">
            {text.welcomeDescription}
          </p>
          
          {prototypeSettings.useSelfSignedCert && !showPasswordInput ? (
            <>
              <div className="w-full max-w-2xl mb-8 p-4 bg-amber-50 border border-amber-200 rounded-lg">
                <div className="flex items-start">
                  <div className="flex-shrink-0">
                    <AlertTriangle className="h-5 w-5 text-amber-400" />
                  </div>
                  <div className="ml-3 text-left">
                    <h3 className="text-sm font-medium text-amber-800">
                      Self-Signed Certificate Warning
                    </h3>
                    <div className="mt-2 text-sm text-amber-700">
                      <p>
                        When you click "Continue", you'll be redirected to a secure HTTPS connection. 
                        Your browser will show a security warning because this wizard uses a self-signed certificate.
                      </p>
                      <p className="mt-2">
                        To proceed:
                      </p>
                      <ol className="list-decimal list-inside mt-1 space-y-1">
                        <li>Click "Advanced" or "Show Details" in your browser's warning</li>
                        <li>Choose "Proceed" or "Continue" to the site</li>
                        <li>You'll return to this page to enter your password</li>
                      </ol>
                    </div>
                  </div>
                </div>
              </div>

              <Button 
                onClick={handleSubmit} 
                size="lg" 
                icon={<ChevronRight className="w-5 h-5" />}
                disabled={isLoading}
              >
                Continue Securely
              </Button>
            </>
          ) : (
            <div className="w-full max-w-sm mb-8">
              <Input
                id="password"
                label="Enter Password"
                type="password"
                value={password}
                onChange={handlePasswordChange}
                onKeyDown={handleKeyDown}
                error={error}
                required
                icon={<Lock className="w-5 h-5" />}
              />

              <Button 
                onClick={handleSubmit} 
                size="lg" 
                className="w-full mt-4"
                icon={<ChevronRight className="w-5 h-5" />}
                disabled={isLoading}
              >
                {text.welcomeButtonText}
              </Button>
            </div>
          )}
        </div>
      </Card>
    </div>
  );
};

export default WelcomeStep;