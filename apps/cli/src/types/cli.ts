import type { Command } from "commander";

export type CommandRegistrar = (program: Command) => void;
