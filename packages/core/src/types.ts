
export interface UniversalRequest {
  method: string;
  path: string;
  headers: Record<string, string>;
  query: Record<string, string | string[]>;
  body?: string;
  raw?: unknown;
}

export interface UniversalResponse {
  status: number;
  headers?: Record<string, string>;
  body?: string;
}

export type UniversalHandler = (
  req: UniversalRequest
) => Promise<UniversalResponse>;
