
import { UniversalHandler } from "@runfabric/core";

export const handler: UniversalHandler = async () => {
  return {
    status: 200,
    headers: {"content-type":"application/json"},
    body: JSON.stringify({message:"hello world"})
  };
};
