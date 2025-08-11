import React, { useState } from "react";
import Button from "../../../common/Button";
import { NextButtonConfig } from "../types";
import { ChevronRight } from "lucide-react";

// HOC that provides setNextButtonConfig prop and renders a button
export function withTestButton<P extends { setNextButtonConfig: (config: NextButtonConfig) => void }>(
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