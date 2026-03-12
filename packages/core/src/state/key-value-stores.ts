import { createRequire } from "node:module";
import type { StateBackendType } from "../project";
import type { ResolvedStateConfig } from "../state";
const requireModule = createRequire(__filename);
export interface KeyValueStore {
  get(key: string): Promise<string | null>;
  put(key: string, value: string): Promise<void>;
  putIfAbsent?(key: string, value: string): Promise<boolean>;
  delete(key: string): Promise<void>;
  list(prefix: string): Promise<string[]>;
}
function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
function requireOptionalModule<T>(moduleName: string, installHint: string): T {
  try {
    return requireModule(moduleName) as T;
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code === "MODULE_NOT_FOUND") {
      throw new Error(`missing optional dependency "${moduleName}". ${installHint}`);
    }
    throw error;
  }
}
function isErrnoWithCode(error: unknown, code: string): boolean {
  return isRecord(error) && typeof error.code === "string" && error.code === code;
}
function isHttpStatusError(error: unknown, statusCode: number): boolean {
  return (
    isRecord(error) &&
    ((typeof error.statusCode === "number" && Number.isFinite(error.statusCode) && error.statusCode === statusCode) ||
      (isRecord(error.$metadata) &&
        typeof error.$metadata.httpStatusCode === "number" &&
        Number.isFinite(error.$metadata.httpStatusCode) &&
        error.$metadata.httpStatusCode === statusCode))
  );
}
function quotePostgresIdentifier(value: string, field: string): string {
  if (!/^[a-zA-Z_][a-zA-Z0-9_]*$/.test(value)) {
    throw new Error(
      `invalid postgres identifier for ${field}: "${value}". Use letters, numbers, underscore and start with a letter or underscore.`
    );
  }
  return `"${value}"`;
}
function formatBackendCredentialError(backend: StateBackendType, detail: string): Error {
  return new Error(`state backend "${backend}" credential/config error: ${detail}`);
}
async function streamToUtf8(input: unknown): Promise<string> {
  if (!input) {
    return "";
  }
  if (typeof input === "string") {
    return input;
  }
  if (Buffer.isBuffer(input)) {
    return input.toString("utf8");
  }
  if (isRecord(input) && typeof input.transformToString === "function") {
    const value = await input.transformToString("utf8");
    return typeof value === "string" ? value : String(value ?? "");
  }

  if (isRecord(input) && typeof input.getReader === "function") {
    const reader = input.getReader();
    const chunks: Uint8Array[] = [];
    while (true) {
      const { done, value } = await reader.read();
      if (done) {
        break;
      }
      chunks.push(value);
    }
    return Buffer.concat(chunks.map((chunk) => Buffer.from(chunk))).toString("utf8");
  }
  const maybeAsyncIterable = input as { [Symbol.asyncIterator]?: () => AsyncIterator<unknown> };
  if (
    typeof input === "object" &&
    input !== null &&
    typeof maybeAsyncIterable[Symbol.asyncIterator] === "function"
  ) {
    const chunks: Buffer[] = [];
    for await (const chunk of maybeAsyncIterable as unknown as AsyncIterable<unknown>) {
      if (typeof chunk === "string") {
        chunks.push(Buffer.from(chunk));
      } else if (chunk instanceof Uint8Array) {
        chunks.push(Buffer.from(chunk));
      } else if (Buffer.isBuffer(chunk)) {
        chunks.push(chunk);
      }
    }
    return Buffer.concat(chunks).toString("utf8");
  }

  return String(input);
}

export class PostgresKeyValueStore implements KeyValueStore {
  private readonly connectionStringEnv: string;
  private readonly schema: string;
  private readonly table: string;
  private readonly tableRef: string;
  private pool:
    | {
        query: (
          queryText: string,
          values?: unknown[]
        ) => Promise<{ rows: Array<Record<string, unknown>>; rowCount: number | null }>;
      }
    | undefined;
  private initPromise: Promise<void> | undefined;

  constructor(config: ResolvedStateConfig) {
    this.connectionStringEnv = config.postgres.connectionStringEnv;
    this.schema = config.postgres.schema;
    this.table = config.postgres.table;
    const schemaSql = quotePostgresIdentifier(this.schema, "state.postgres.schema");
    const tableSql = quotePostgresIdentifier(this.table, "state.postgres.table");
    this.tableRef = `${schemaSql}.${tableSql}`;
  }

  private async init(): Promise<void> {
    if (this.initPromise) {
      return this.initPromise;
    }

    this.initPromise = (async () => {
      const connectionString = process.env[this.connectionStringEnv];
      if (!connectionString || connectionString.trim().length === 0) {
        throw formatBackendCredentialError(
          "postgres",
          `set ${this.connectionStringEnv} with a valid postgres connection string`
        );
      }

      const pg = requireOptionalModule<{ Pool: new (options: { connectionString: string }) => unknown }>(
        "pg",
        "Install with: pnpm add -w --filter @runfabric/core pg"
      );
      const poolCandidate = new pg.Pool({ connectionString });
      if (!isRecord(poolCandidate) || typeof poolCandidate.query !== "function") {
        throw new Error("pg Pool initialization failed");
      }
      this.pool = poolCandidate as {
        query: (
          queryText: string,
          values?: unknown[]
        ) => Promise<{ rows: Array<Record<string, unknown>>; rowCount: number | null }>;
      };

      await this.pool.query(
        `CREATE SCHEMA IF NOT EXISTS ${quotePostgresIdentifier(this.schema, "state.postgres.schema")}`
      );
      await this.pool.query(
        `CREATE TABLE IF NOT EXISTS ${this.tableRef} (
          key TEXT PRIMARY KEY,
          value TEXT NOT NULL,
          updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
        )`
      );
    })();

    return this.initPromise;
  }

  private async query(
    queryText: string,
    values?: unknown[]
  ): Promise<{ rows: Array<Record<string, unknown>>; rowCount: number | null }> {
    await this.init();
    if (!this.pool) {
      throw new Error("postgres pool was not initialized");
    }
    return this.pool.query(queryText, values);
  }

  async get(key: string): Promise<string | null> {
    const result = await this.query(`SELECT value FROM ${this.tableRef} WHERE key = $1`, [key]);
    if (result.rows.length === 0) {
      return null;
    }
    const value = result.rows[0].value;
    return typeof value === "string" ? value : null;
  }

  async put(key: string, value: string): Promise<void> {
    await this.query(
      `INSERT INTO ${this.tableRef} (key, value, updated_at)
       VALUES ($1, $2, now())
       ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`,
      [key, value]
    );
  }

  async putIfAbsent(key: string, value: string): Promise<boolean> {
    const result = await this.query(
      `INSERT INTO ${this.tableRef} (key, value, updated_at)
       VALUES ($1, $2, now())
       ON CONFLICT (key) DO NOTHING`,
      [key, value]
    );
    return (result.rowCount || 0) > 0;
  }

  async delete(key: string): Promise<void> {
    await this.query(`DELETE FROM ${this.tableRef} WHERE key = $1`, [key]);
  }

  async list(prefix: string): Promise<string[]> {
    const likePrefix = `${prefix}%`;
    const result = await this.query(`SELECT key FROM ${this.tableRef} WHERE key LIKE $1 ORDER BY key ASC`, [
      likePrefix
    ]);
    return result.rows
      .map((row) => row.key)
      .filter((value): value is string => typeof value === "string");
  }
}

export class S3KeyValueStore implements KeyValueStore {
  private readonly bucket: string;
  private readonly client: { send: (command: unknown) => Promise<Record<string, unknown>> };
  private readonly commands: {
    GetObjectCommand: new (input: Record<string, unknown>) => unknown;
    PutObjectCommand: new (input: Record<string, unknown>) => unknown;
    DeleteObjectCommand: new (input: Record<string, unknown>) => unknown;
    ListObjectsV2Command: new (input: Record<string, unknown>) => unknown;
  };

  constructor(config: ResolvedStateConfig) {
    if (!config.s3.bucket) {
      throw formatBackendCredentialError("s3", "set state.s3.bucket in runfabric.yml");
    }
    this.bucket = config.s3.bucket;

    const aws = requireOptionalModule<{
      S3Client: new (options: Record<string, unknown>) => { send: (command: unknown) => Promise<Record<string, unknown>> };
      GetObjectCommand: new (input: Record<string, unknown>) => unknown;
      PutObjectCommand: new (input: Record<string, unknown>) => unknown;
      DeleteObjectCommand: new (input: Record<string, unknown>) => unknown;
      ListObjectsV2Command: new (input: Record<string, unknown>) => unknown;
    }>("@aws-sdk/client-s3", "Install with: pnpm add -w --filter @runfabric/core @aws-sdk/client-s3");

    this.client = new aws.S3Client({
      ...(config.s3.region ? { region: config.s3.region } : {})
    });
    this.commands = {
      GetObjectCommand: aws.GetObjectCommand,
      PutObjectCommand: aws.PutObjectCommand,
      DeleteObjectCommand: aws.DeleteObjectCommand,
      ListObjectsV2Command: aws.ListObjectsV2Command
    };
  }

  async get(key: string): Promise<string | null> {
    try {
      const response = await this.client.send(
        new this.commands.GetObjectCommand({
          Bucket: this.bucket,
          Key: key
        })
      );
      return streamToUtf8(response.Body);
    } catch (error) {
      if (
        isErrnoWithCode(error, "NoSuchKey") ||
        isErrnoWithCode(error, "NotFound") ||
        isHttpStatusError(error, 404) ||
        (isRecord(error) &&
          ((typeof error.name === "string" && (error.name === "NoSuchKey" || error.name === "NotFound")) ||
            (typeof error.message === "string" &&
              (error.message.toLowerCase().includes("specified key does not exist") ||
                error.message.toLowerCase().includes("no such key")))))
      ) {
        return null;
      }
      throw error;
    }
  }

  async put(key: string, value: string): Promise<void> {
    await this.client.send(
      new this.commands.PutObjectCommand({
        Bucket: this.bucket,
        Key: key,
        Body: value,
        ContentType: "application/json; charset=utf-8"
      })
    );
  }

  async putIfAbsent(key: string, value: string): Promise<boolean> {
    try {
      await this.client.send(
        new this.commands.PutObjectCommand({
          Bucket: this.bucket,
          Key: key,
          Body: value,
          ContentType: "application/json; charset=utf-8",
          IfNoneMatch: "*"
        })
      );
      return true;
    } catch (error) {
      if (isErrnoWithCode(error, "PreconditionFailed") || isHttpStatusError(error, 412)) {
        return false;
      }
      throw error;
    }
  }

  async delete(key: string): Promise<void> {
    await this.client.send(
      new this.commands.DeleteObjectCommand({
        Bucket: this.bucket,
        Key: key
      })
    );
  }

  async list(prefix: string): Promise<string[]> {
    const out: string[] = [];
    let continuationToken: string | undefined;
    do {
      const response = await this.client.send(
        new this.commands.ListObjectsV2Command({
          Bucket: this.bucket,
          Prefix: prefix,
          ContinuationToken: continuationToken
        })
      );
      const contents = Array.isArray(response.Contents) ? response.Contents : [];
      for (const entry of contents) {
        if (isRecord(entry) && typeof entry.Key === "string") {
          out.push(entry.Key);
        }
      }
      continuationToken =
        typeof response.NextContinuationToken === "string" ? response.NextContinuationToken : undefined;
    } while (continuationToken);
    return out;
  }
}

export class GcsKeyValueStore implements KeyValueStore {
  private readonly bucket: {
    file: (name: string) => {
      download: () => Promise<[Buffer]>;
      save: (data: string, options?: Record<string, unknown>) => Promise<void>;
      delete: (options?: Record<string, unknown>) => Promise<void>;
    };
    getFiles: (options?: Record<string, unknown>) => Promise<[Array<{ name: string }>, Record<string, unknown>?]>;
  };

  constructor(config: ResolvedStateConfig) {
    if (!config.gcs.bucket) {
      throw formatBackendCredentialError("gcs", "set state.gcs.bucket in runfabric.yml");
    }
    const gcs = requireOptionalModule<{ Storage: new () => { bucket: (name: string) => unknown } }>(
      "@google-cloud/storage",
      "Install with: pnpm add -w --filter @runfabric/core @google-cloud/storage"
    );
    const storage = new gcs.Storage();
    const bucketCandidate = storage.bucket(config.gcs.bucket);
    if (!isRecord(bucketCandidate) || typeof bucketCandidate.file !== "function") {
      throw new Error("gcs bucket initialization failed");
    }
    this.bucket = bucketCandidate as {
      file: (name: string) => {
        download: () => Promise<[Buffer]>;
        save: (data: string, options?: Record<string, unknown>) => Promise<void>;
        delete: (options?: Record<string, unknown>) => Promise<void>;
      };
      getFiles: (
        options?: Record<string, unknown>
      ) => Promise<[Array<{ name: string }>, Record<string, unknown>?]>;
    };
  }

  async get(key: string): Promise<string | null> {
    const file = this.bucket.file(key);
    try {
      const [content] = await file.download();
      return content.toString("utf8");
    } catch (error) {
      if (isErrnoWithCode(error, "404") || isHttpStatusError(error, 404)) {
        return null;
      }
      throw error;
    }
  }

  async put(key: string, value: string): Promise<void> {
    const file = this.bucket.file(key);
    await file.save(value, {
      resumable: false,
      contentType: "application/json; charset=utf-8"
    });
  }

  async putIfAbsent(key: string, value: string): Promise<boolean> {
    const file = this.bucket.file(key);
    try {
      await file.save(value, {
        resumable: false,
        contentType: "application/json; charset=utf-8",
        preconditionOpts: {
          ifGenerationMatch: 0
        }
      });
      return true;
    } catch (error) {
      if (isErrnoWithCode(error, "412") || isHttpStatusError(error, 412)) {
        return false;
      }
      throw error;
    }
  }

  async delete(key: string): Promise<void> {
    await this.bucket.file(key).delete({
      ignoreNotFound: true
    });
  }

  async list(prefix: string): Promise<string[]> {
    const keys: string[] = [];
    let pageToken: string | undefined;
    do {
      const [files, nextQuery] = await this.bucket.getFiles({
        prefix,
        pageToken,
        autoPaginate: false
      });
      for (const file of files) {
        if (file && typeof file.name === "string") {
          keys.push(file.name);
        }
      }
      pageToken =
        nextQuery && typeof nextQuery.pageToken === "string" ? nextQuery.pageToken : undefined;
    } while (pageToken);

    return keys;
  }
}

type AzBlobServiceClient = {
  getContainerClient: (containerName: string) => unknown;
};
type AzBlobModule = {
  BlobServiceClient: {
    fromConnectionString: (value: string) => AzBlobServiceClient;
    new (url: string, credential: unknown): AzBlobServiceClient;
  };
  StorageSharedKeyCredential: new (accountName: string, accountKey: string) => unknown;
};
function loadAzBlobModule(): AzBlobModule {
  return requireOptionalModule<AzBlobModule>(
    "@azure/storage-blob",
    "Install with: pnpm add -w --filter @runfabric/core @azure/storage-blob"
  );
}
function createAzBlobServiceClient(az: AzBlobModule): AzBlobServiceClient {
  const connectionString = process.env.AZURE_STORAGE_CONNECTION_STRING;
  if (connectionString && connectionString.trim().length > 0) {
    return az.BlobServiceClient.fromConnectionString(connectionString.trim());
  }
  const account = process.env.AZURE_STORAGE_ACCOUNT;
  const key = process.env.AZURE_STORAGE_KEY;
  if (!account || !key) {
    throw formatBackendCredentialError(
      "azblob",
      "set AZURE_STORAGE_CONNECTION_STRING or AZURE_STORAGE_ACCOUNT + AZURE_STORAGE_KEY"
    );
  }
  const credential = new az.StorageSharedKeyCredential(account, key);
  const candidate = new az.BlobServiceClient(`https://${account}.blob.core.windows.net`, credential);
  if (!isRecord(candidate) || typeof candidate.getContainerClient !== "function") {
    throw new Error("azblob service client initialization failed");
  }
  return candidate as AzBlobServiceClient;
}
function createAzBlobContainerClient(
  serviceClient: AzBlobServiceClient,
  containerName: string
): {
  createIfNotExists: () => Promise<unknown>;
  getBlockBlobClient: (name: string) => {
    upload: (data: string, length: number, options?: Record<string, unknown>) => Promise<unknown>;
    deleteIfExists: () => Promise<unknown>;
    download: () => Promise<{ readableStreamBody?: unknown }>;
  };
  listBlobsFlat: (options?: Record<string, unknown>) => AsyncIterable<{ name?: string }>;
} {
  const containerCandidate = serviceClient.getContainerClient(containerName);
  if (!isRecord(containerCandidate) || typeof containerCandidate.getBlockBlobClient !== "function") {
    throw new Error("azblob container client initialization failed");
  }
  return containerCandidate as {
    createIfNotExists: () => Promise<unknown>;
    getBlockBlobClient: (name: string) => {
      upload: (data: string, length: number, options?: Record<string, unknown>) => Promise<unknown>;
      deleteIfExists: () => Promise<unknown>;
      download: () => Promise<{ readableStreamBody?: unknown }>;
    };
    listBlobsFlat: (options?: Record<string, unknown>) => AsyncIterable<{ name?: string }>;
  };
}
export class AzBlobKeyValueStore implements KeyValueStore {
  private readonly container: {
    createIfNotExists: () => Promise<unknown>;
    getBlockBlobClient: (name: string) => {
      upload: (data: string, length: number, options?: Record<string, unknown>) => Promise<unknown>;
      deleteIfExists: () => Promise<unknown>;
      download: () => Promise<{ readableStreamBody?: unknown }>;
    };
    listBlobsFlat: (options?: Record<string, unknown>) => AsyncIterable<{ name?: string }>;
  };
  private ensureContainerPromise: Promise<void> | undefined;

  constructor(config: ResolvedStateConfig) {
    if (!config.azblob.container) {
      throw formatBackendCredentialError("azblob", "set state.azblob.container in runfabric.yml");
    }
    const az = loadAzBlobModule();
    const serviceClient = createAzBlobServiceClient(az);
    this.container = createAzBlobContainerClient(serviceClient, config.azblob.container);
  }
  private async ensureContainer(): Promise<void> {
    if (!this.ensureContainerPromise) {
      this.ensureContainerPromise = (async () => {
        await this.container.createIfNotExists();
      })();
    }
    await this.ensureContainerPromise;
  }

  async get(key: string): Promise<string | null> {
    await this.ensureContainer();
    const blob = this.container.getBlockBlobClient(key);
    try {
      const response = await blob.download();
      return streamToUtf8(response.readableStreamBody);
    } catch (error) {
      if (isHttpStatusError(error, 404) || isErrnoWithCode(error, "BlobNotFound")) {
        return null;
      }
      throw error;
    }
  }

  async put(key: string, value: string): Promise<void> {
    await this.ensureContainer();
    const blob = this.container.getBlockBlobClient(key);
    await blob.upload(value, Buffer.byteLength(value), {
      blobHTTPHeaders: {
        blobContentType: "application/json; charset=utf-8"
      }
    });
  }

  async putIfAbsent(key: string, value: string): Promise<boolean> {
    await this.ensureContainer();
    const blob = this.container.getBlockBlobClient(key);
    try {
      await blob.upload(value, Buffer.byteLength(value), {
        blobHTTPHeaders: {
          blobContentType: "application/json; charset=utf-8"
        },
        conditions: {
          ifNoneMatch: "*"
        }
      });
      return true;
    } catch (error) {
      if (isHttpStatusError(error, 412) || isErrnoWithCode(error, "ConditionNotMet")) {
        return false;
      }
      throw error;
    }
  }

  async delete(key: string): Promise<void> {
    await this.ensureContainer();
    await this.container.getBlockBlobClient(key).deleteIfExists();
  }

  async list(prefix: string): Promise<string[]> {
    await this.ensureContainer();
    const out: string[] = [];
    for await (const blob of this.container.listBlobsFlat({ prefix })) {
      if (blob && typeof blob.name === "string") {
        out.push(blob.name);
      }
    }
    return out;
  }
}
