import React, { useState, useCallback } from "react";
import Button from "../../../common/Button";
import { NextButtonConfig, BackButtonConfig } from "../types";
import { ChevronRight, ChevronLeft } from "lucide-react";

// HOC that provides setNextButtonConfig, setBackButtonConfig, and onBack props and renders buttons
export function withTestButton<P extends {
  setNextButtonConfig: (config: NextButtonConfig) => void;
  setBackButtonConfig: (config: BackButtonConfig) => void;
  onBack: () => void;
}>(
  Component: React.ComponentType<P>
) {
  return function WrappedComponent(props: Omit<P, 'setNextButtonConfig' | 'setBackButtonConfig' | 'onBack'>) {
    const [nextButtonConfig, setNextButtonConfig] = useState<NextButtonConfig>({
      disabled: true,
      onClick: () => {}
    });

    const [backButtonConfig, setBackButtonConfig] = useState<BackButtonConfig>({
      hidden: true,
      onClick: () => {}
    });

    const mockOnBack = useCallback(() => {
      console.log('Back button clicked');
    }, []);

    return (
      <div>
        <Component
          {...(props as P)}
          setNextButtonConfig={setNextButtonConfig}
          setBackButtonConfig={setBackButtonConfig}
          onBack={mockOnBack}
        />
        <div className="flex justify-between">
          {!backButtonConfig.hidden && (
            <Button
              onClick={backButtonConfig.onClick}
              variant="outline"
              disabled={backButtonConfig.disabled ?? false}
              icon={<ChevronLeft className="w-5 h-5" />}
              dataTestId="back-button"
            >
              Back
            </Button>
          )}
          <Button
            onClick={nextButtonConfig.onClick}
            disabled={nextButtonConfig.disabled}
            icon={<ChevronRight className="w-5 h-5" />}
            dataTestId="next-button"
            className={backButtonConfig.hidden ? 'ml-auto' : ''}
          >
            Next
          </Button>
        </div>
      </div>
    );
  };
}

// HOC that provides setNextButtonConfig and renders next button
export function withNextButtonOnly<P extends {
  setNextButtonConfig: (config: NextButtonConfig) => void;
}>(
  Component: React.ComponentType<P>
) {
  return function WrappedComponent(props: Omit<P, 'setNextButtonConfig'>) {
    const [nextButtonConfig, setNextButtonConfig] = useState<NextButtonConfig>({
      disabled: true,
      onClick: () => {}
    });

    return (
      <div>
        <Component
          {...(props as P)}
          setNextButtonConfig={setNextButtonConfig}
        />
        <div className="flex justify-end">
          <Button
            onClick={nextButtonConfig.onClick}
            disabled={nextButtonConfig.disabled}
            icon={<ChevronRight className="w-5 h-5" />}
            dataTestId="next-button"
          >
            Next
          </Button>
        </div>
      </div>
    );
  };
}
