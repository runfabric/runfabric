export async function resolve(specifier, context, defaultResolve) {
  if (specifier.startsWith("@runfabric/")) {
    const packageName = specifier.slice("@runfabric/".length);
    const packagePath =
      packageName === "cli"
        ? new URL("../apps/cli/src/index.ts", import.meta.url)
        : new URL(`../packages/${packageName}/src/index.ts`, import.meta.url);

    return {
      url: packagePath.href,
      shortCircuit: true
    };
  }

  try {
    return await defaultResolve(specifier, context, defaultResolve);
  } catch (error) {
    if (
      error &&
      error.code === "ERR_MODULE_NOT_FOUND" &&
      (specifier.startsWith("./") || specifier.startsWith("../")) &&
      !specifier.endsWith(".ts")
    ) {
      return defaultResolve(`${specifier}.ts`, context, defaultResolve);
    }
    throw error;
  }
}
