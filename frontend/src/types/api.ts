export interface User {
  id: number;
  created_at: string;
  updated_at: string;
  username: string;
}

export interface Todo {
  id: number;
  created_at: string;
  updated_at: string;
  title: string;
  description: string;
  completed: boolean;
  user_id: number;
  user?: User;
}

export interface LoginResponse {
  token: string;
  user: User;
}

export interface ApiErrorBody {
  error?: string;
  message?: string;
}
