import { describe, it, expect, vi } from "vitest";
import { screen } from "@testing-library/react";
import { renderWithProviders } from "../../../test/setup.tsx";
import StepNavigation from "../StepNavigation.tsx";
import { WizardStep } from "../../../types/index.ts";

describe("StepNavigation", () => {
  const defaultContextValues = {
    settingsContext: {
      settings: {
        themeColor: "#316DE6",
      },
      updateSettings: vi.fn(),
    },
  };

  describe("Linux Target", () => {
    it("shows 'current' status for the current step", () => {
      renderWithProviders(<StepNavigation currentStep="linux-setup" />, {
        wrapperProps: {
          authenticated: true,
          contextValues: defaultContextValues,
        },
      });

      const setupStep = screen.getByText("Setup").closest("div");
      expect(setupStep).toHaveStyle({
        border: "1px solid #316DE6",
      });
    });

    it("shows upcoming steps with default styling", () => {
      renderWithProviders(<StepNavigation currentStep="welcome" />, {
        wrapperProps: {
          authenticated: true,
          contextValues: defaultContextValues,
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
          contextValues: defaultContextValues,
        },
      });

      // Check that all step icons are rendered
      const stepElements = screen.getAllByRole("listitem");
      expect(stepElements).toHaveLength(5); // welcome, setup, configuration, installation, completion

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
        { currentStep: "linux-setup", setupStatus: "current", installStatus: "upcoming" },
        { currentStep: "installation", setupStatus: "complete", installStatus: "current" },
      ];

      testCases.forEach(({ currentStep, setupStatus, installStatus }) => {
        const { unmount } = renderWithProviders(
          <StepNavigation currentStep={currentStep as WizardStep} />,
          {
            wrapperProps: {
              authenticated: true,
              contextValues: defaultContextValues,
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

  describe("Kubernetes Target", () => {
    it("renders all navigation steps", () => {
      renderWithProviders(<StepNavigation currentStep="welcome" />, {
        wrapperProps: {
          authenticated: true,
          contextValues: defaultContextValues,
          target: "kubernetes",
        },
      });

      // Should show 4 steps (welcome, setup, installation, completion)
      expect(screen.getByText("Welcome")).toBeInTheDocument();
      expect(screen.getByText("Setup")).toBeInTheDocument();
      expect(screen.getByText("Installation")).toBeInTheDocument();
      expect(screen.getByText("Completion")).toBeInTheDocument();
    });

    it("shows 'current' status for the current step", () => {
      renderWithProviders(<StepNavigation currentStep="kubernetes-setup" />, {
        wrapperProps: {
          authenticated: true,
          contextValues: defaultContextValues,
          target: "kubernetes",
        },
      });

      const setupStep = screen.getByText("Setup").closest("div");
      expect(setupStep).toHaveStyle({
        border: "1px solid #316DE6",
      });
    });

    it("shows upcoming steps with default styling", () => {
      renderWithProviders(<StepNavigation currentStep="welcome" />, {
        wrapperProps: {
          authenticated: true,
          contextValues: defaultContextValues,
          target: "kubernetes",
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
          contextValues: defaultContextValues,
          target: "kubernetes",
        },
      });

      // Check that all step icons are rendered
      const stepElements = screen.getAllByRole("listitem");
      expect(stepElements).toHaveLength(5); // welcome, setup, configuration, installation, completion

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
        { currentStep: "kubernetes-setup", setupStatus: "current", installStatus: "upcoming" },
        { currentStep: "installation", setupStatus: "complete", installStatus: "current" },
      ];

      testCases.forEach(({ currentStep, setupStatus, installStatus }) => {
        const { unmount } = renderWithProviders(
          <StepNavigation currentStep={currentStep as WizardStep} />,
          {
            wrapperProps: {
              authenticated: true,
              contextValues: defaultContextValues,
              target: "kubernetes",
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
});
