import type { ApiErrorBody, LoginResponse, Todo } from "@/types/api";

const API_BASE_URL = (import.meta.env.VITE_API_BASE_URL ?? "").replace(/\/$/, "");

export class ApiError extends Error {
  constructor(
    message: string,
    public readonly status: number,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

export interface ApiClientOptions {
  getToken: () => string | null;
  onUnauthorized: () => void;
}

export function createApiClient({ getToken, onUnauthorized }: ApiClientOptions) {
  async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
    const headers = new Headers(options.headers);
    const token = getToken();

    if (options.body && !headers.has("Content-Type")) {
      headers.set("Content-Type", "application/json");
    }

    if (token) {
      headers.set("Authorization", `Bearer ${token}`);
    }

    const response = await fetch(`${API_BASE_URL}${path}`, {
      ...options,
      headers,
    });

    if (response.status === 401) {
      onUnauthorized();
    }

    if (!response.ok) {
      throw new ApiError(await readError(response), response.status);
    }

    if (response.status === 204) {
      return undefined as T;
    }

    return (await response.json()) as T;
  }

  return {
    login(username: string, password: string) {
      return request<LoginResponse>("/api/login", {
        method: "POST",
        body: JSON.stringify({ username, password }),
      });
    },
    register(username: string, password: string) {
      return request<{ message: string }>("/api/register", {
        method: "POST",
        body: JSON.stringify({ username, password }),
      });
    },
    listTodos() {
      return request<Todo[]>("/api/todos");
    },
    createTodo(payload: Pick<Todo, "title" | "description">) {
      return request<Todo>("/api/todos", {
        method: "POST",
        body: JSON.stringify(payload),
      });
    },
    updateTodo(id: number, payload: Partial<Pick<Todo, "title" | "description" | "completed">>) {
      return request<Todo>(`/api/todos/${id}`, {
        method: "PUT",
        body: JSON.stringify(payload),
      });
    },
    deleteTodo(id: number) {
      return request<{ message: string }>(`/api/todos/${id}`, {
        method: "DELETE",
      });
    },
  };
}

async function readError(response: Response) {
  try {
    const body = (await response.json()) as ApiErrorBody;
    return body.error ?? body.message ?? `Request failed with status ${response.status}`;
  } catch {
    return `Request failed with status ${response.status}`;
  }
}
