export interface ProjectResources {
  memory?: number;
  timeout?: number;
}

export interface TriggerConfig {
  type: string;
  method?: string;
  path?: string;
  schedule?: string;
  queue?: string;
  [key: string]: string | undefined;
}

export interface FunctionConfig {
  name: string;
  entry?: string;
  runtime?: string;
  triggers?: TriggerConfig[];
  resources?: ProjectResources;
}

export interface ProjectConfig {
  service: string;
  runtime: string;
  entry: string;
  stage?: string;
  providers: string[];
  triggers: TriggerConfig[];
  functions?: FunctionConfig[];
  hooks?: string[];
  resources?: ProjectResources;
  env?: Record<string, string>;
  params?: Record<string, string>;
  extensions?: Record<string, Record<string, unknown>>;
}
