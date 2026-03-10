export interface ProviderCredentialField {
  env: string;
  description: string;
  required?: boolean;
}

export interface ProviderCredentialSchema {
  provider: string;
  fields: ProviderCredentialField[];
}

export interface CredentialEvaluation {
  missingRequired: ProviderCredentialField[];
  missingOptional: ProviderCredentialField[];
}

function hasCredentialValue(value: string | undefined): boolean {
  return typeof value === "string" && value.trim().length > 0;
}

export function evaluateCredentialSchema(
  schema: ProviderCredentialSchema,
  env: Record<string, string | undefined>
): CredentialEvaluation {
  const missingRequired: ProviderCredentialField[] = [];
  const missingOptional: ProviderCredentialField[] = [];

  for (const field of schema.fields) {
    if (hasCredentialValue(env[field.env])) {
      continue;
    }

    if (field.required === false) {
      missingOptional.push(field);
      continue;
    }

    missingRequired.push(field);
  }

  return {
    missingRequired,
    missingOptional
  };
}
