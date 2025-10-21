import { ApiErrorResponse } from "../types";

export class ApiError extends Error {
  statusCode: number;
  details: string = ""; // will hold additional error details from the body response if available
  fieldErrors?: { field: string; message: string }[]; // contains the multiple config errors if that's the case

  constructor(status: number, message: string) {
    super(message);
    this.name = "ApiError";
    this.statusCode = status;
    this.message = message;
  }

  // fromResponse creates an ApiError from a fetch Response object
  // It attempts to parse the response body as JSON to extract additional error details
  // If parsing fails, it ignores the error and returns the ApiError with just the status and message
  static fromResponse = async (
    response: Response,
    message: string,
  ): Promise<ApiError> => {
    const error = new ApiError(response.status, message);
    try {
      const data = (await response.json()) as ApiErrorResponse;
      error.details = data.message;
      error.fieldErrors = data.errors;
    } catch {
      // Ignore JSON parsing errors
    }
    return error;
  };
}

export class ConfigError extends ApiError {}
