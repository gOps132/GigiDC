export interface EmbeddingClient {
  createEmbedding(model: string, input: string): Promise<number[]>;
}

export interface ResponseClient {
  createTextResponse(input: {
    instructions: string;
    model: string;
    text: string;
  }): Promise<string>;
}
