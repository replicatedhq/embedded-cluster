import React from "react";
import { describe, it, expect } from "vitest";
import { screen } from "@testing-library/react";
import { renderWithProviders } from "../../../test/setup.tsx";
import StepNavigation from "../StepNavigation.tsx";
import { WizardStep } from "../../../types/index.ts";

describe("StepNavigation", () => {
  const defaultPreloadedState = {
    // Use generic settings instead of prototype-specific references
    prototypeSettings: {
      themeColor: "#316DE6",
    },
  };

  it("renders all navigation steps except validation", () => {
    renderWithProviders(<StepNavigation currentStep="welcome" />, {
      wrapperProps: {
        authenticated: true,
        preloadedState: defaultPreloadedState,
      },
    });

    // Should show 4 steps (welcome, setup, installation, completion)
    expect(screen.getByText("Welcome")).toBeInTheDocument();
    expect(screen.getByText("Setup")).toBeInTheDocument();
    expect(screen.getByText("Installation")).toBeInTheDocument();
    expect(screen.getByText("Completion")).toBeInTheDocument();

    // Should NOT show validation step
    expect(screen.queryByText("Validation")).not.toBeInTheDocument();
  });

  it("shows 'current' status for the current step", () => {
    renderWithProviders(<StepNavigation currentStep="setup" />, {
      wrapperProps: {
        authenticated: true,
        preloadedState: defaultPreloadedState,
      },
    });

    const setupStep = screen.getByText("Setup").closest("div");
    expect(setupStep).toHaveStyle({
      border: "1px solid #316DE6",
    });
  });

  it("treats validation step as part of setup for navigation", () => {
    renderWithProviders(<StepNavigation currentStep="validation" />, {
      wrapperProps: {
        authenticated: true,
        preloadedState: defaultPreloadedState,
      },
    });

    // When currentStep is 'validation', setup should show as current
    const setupStep = screen.getByText("Setup").closest("div");
    expect(setupStep).toHaveStyle({
      border: "1px solid #316DE6",
    });

    // Welcome should be complete
    const welcomeStep = screen.getByText("Welcome").closest("div");
    expect(welcomeStep).toHaveStyle({
      backgroundColor: "#316DE61A",
      color: "#316DE6",
    });
  });

  it("shows upcoming steps with default styling", () => {
    renderWithProviders(<StepNavigation currentStep="welcome" />, {
      wrapperProps: {
        authenticated: true,
        preloadedState: defaultPreloadedState,
      },
    });

    // Setup, Installation, and Completion should be upcoming
    const installationStep = screen.getByText("Installation").closest("div");
    const completionStep = screen.getByText("Completion").closest("div");

    expect(installationStep).toHaveStyle({
      backgroundColor: "rgb(243 244 246)", // gray background
      color: "rgb(107 114 128)", // gray text
    });
    expect(completionStep).toHaveStyle({
      backgroundColor: "rgb(243 244 246)",
      color: "rgb(107 114 128)",
    });
  });

  it("renders correct icons for each step", () => {
    renderWithProviders(<StepNavigation currentStep="welcome" />, {
      wrapperProps: {
        authenticated: true,
        preloadedState: defaultPreloadedState,
      },
    });

    // Check that all step icons are rendered
    const stepElements = screen.getAllByRole("listitem");
    expect(stepElements).toHaveLength(4); // welcome, setup, installation, completion

    // Each step should have an icon (svg element)
    stepElements.forEach((step) => {
      const icon = step.querySelector("svg");
      expect(icon).toBeInTheDocument();
      expect(icon).toHaveClass("w-5", "h-5");
    });
  });

  it("maintains proper step progression logic", () => {
    // Test different current steps and their expected status
    const testCases = [
      { currentStep: "welcome", setupStatus: "upcoming", installStatus: "upcoming" },
      { currentStep: "setup", setupStatus: "current", installStatus: "upcoming" },
      { currentStep: "validation", setupStatus: "current", installStatus: "upcoming" },
      { currentStep: "installation", setupStatus: "complete", installStatus: "current" },
    ];

    testCases.forEach(({ currentStep, setupStatus, installStatus }) => {
      const { unmount } = renderWithProviders(
        <StepNavigation currentStep={currentStep as WizardStep} />,
        {
          wrapperProps: {
            authenticated: true,
            preloadedState: defaultPreloadedState,
          },
        }
      );

      const setupStep = screen.getByText("Setup").closest("div");
      const installStep = screen.getByText("Installation").closest("div");

      if (setupStatus === "current") {
        expect(setupStep).toHaveStyle({ border: "1px solid #316DE6" });
      } else if (setupStatus === "complete") {
        expect(setupStep).toHaveStyle({ 
          backgroundColor: "#316DE61A",
          color: "#316DE6" 
        });
      }

      if (installStatus === "current") {
        expect(installStep).toHaveStyle({ border: "1px solid #316DE6" });
      } else if (installStatus === "upcoming") {
        expect(installStep).toHaveStyle({ 
          backgroundColor: "rgb(243 244 246)",
          color: "rgb(107 114 128)" 
        });
      }

      unmount(); // Clean up for next iteration
    });
  });
});
