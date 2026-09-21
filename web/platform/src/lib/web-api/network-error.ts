// Transport failures are distinct from HTTP errors, validation and cancellation.
export class WebNetworkError extends Error {
  constructor() {
    super("Unable to complete the request.");
    this.name = "WebNetworkError";
  }
}
