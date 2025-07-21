import { createContext } from "react";
import { InitialState } from "../../types";

export const InitialStateContext = createContext<InitialState>({ title: "My App", installTarget: "linux" });
