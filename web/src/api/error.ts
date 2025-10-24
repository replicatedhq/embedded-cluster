import type { components } from "../types/api";

type ApiErrorResponse = components["schemas"]["types.APIError"];

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
      error.fieldErrors = convertToFieldErrors(data.errors);
    } catch {
      // Ignore JSON parsing errors
    }
    return error;
  };
}

// convertToFieldErrors converts the ApiErrorResponse.errors array
// into a flat array of field errors with field and message properties
const convertToFieldErrors = (
  errors?: ApiErrorResponse[],
): { field: string; message: string }[] | undefined => {
  if (!errors || errors.length === 0) {
    return undefined;
  }

  const fieldErrors = errors
    .filter((error) => error.field !== undefined)
    .map((error) => ({
      field: error.field!,
      message: error.message,
    }));

  return fieldErrors.length > 0 ? fieldErrors : undefined;
};
