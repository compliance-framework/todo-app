<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import {
  Check,
  Circle,
  LogIn,
  LogOut,
  Pencil,
  Plus,
  RefreshCw,
  Save,
  ShieldCheck,
  Trash2,
  UserPlus,
  X,
} from "lucide-vue-next";

import Badge from "@/components/ui/Badge.vue";
import Button from "@/components/ui/Button.vue";
import Card from "@/components/ui/Card.vue";
import Input from "@/components/ui/Input.vue";
import Label from "@/components/ui/Label.vue";
import Textarea from "@/components/ui/Textarea.vue";
import { ApiError, createApiClient } from "@/lib/api";
import type { Todo, User } from "@/types/api";

const TOKEN_KEY = "todo-app-token";
const USER_KEY = "todo-app-user";

const oidcLoginUrl = import.meta.env.VITE_OIDC_LOGIN_URL ?? "";

const token = ref<string | null>(localStorage.getItem(TOKEN_KEY));
const currentUser = ref<User | null>(readStoredUser());
const authMode = ref<"login" | "register">("login");
const todos = ref<Todo[]>([]);
const loadingTodos = ref(false);
const authLoading = ref(false);
const savingTodo = ref(false);
const errorMessage = ref("");
const statusMessage = ref("");
const editingTodoId = ref<number | null>(null);

const authForm = reactive({
  username: "",
  password: "",
});

const todoForm = reactive({
  title: "",
  description: "",
});

const editForm = reactive({
  title: "",
  description: "",
});

const api = createApiClient({
  getToken: () => token.value,
  onUnauthorized: handleUnauthorized,
});

const completedCount = computed(() => todos.value.filter((todo) => todo.completed).length);
const openCount = computed(() => todos.value.length - completedCount.value);

onMounted(() => {
  void loadTodos();
});

async function loadTodos() {
  loadingTodos.value = true;
  clearMessages();

  try {
    todos.value = await api.listTodos();
  } catch (error) {
    setError(error, "Unable to load todos.");
  } finally {
    loadingTodos.value = false;
  }
}

async function submitAuth() {
  clearMessages();

  const username = authForm.username.trim();

  if (!username) {
    errorMessage.value = "Username is required.";
    return;
  }

  if (authMode.value === "register" && username.length < 3) {
    errorMessage.value = "Username must be at least 3 characters.";
    return;
  }

  authLoading.value = true;

  try {
    if (authMode.value === "register") {
      await api.register(username, authForm.password);
      statusMessage.value = "Account created.";
    }

    const response = await api.login(username, authForm.password);
    setSession(response.token, response.user);
    authForm.password = "";
    statusMessage.value = authMode.value === "register" ? "Account created and signed in." : "Signed in.";
  } catch (error) {
    setError(error, authMode.value === "register" ? "Unable to register." : "Unable to sign in.");
  } finally {
    authLoading.value = false;
  }
}

async function createTodo() {
  if (!currentUser.value) {
    authMode.value = "login";
    errorMessage.value = "Sign in to create todos.";
    return;
  }

  clearMessages();

  const title = todoForm.title.trim();
  const description = todoForm.description.trim();

  if (!title) {
    errorMessage.value = "Todo title is required.";
    return;
  }

  savingTodo.value = true;

  try {
    const createdTodo = await api.createTodo({
      title,
      description,
    });
    todos.value = [createdTodo, ...todos.value];
    todoForm.title = "";
    todoForm.description = "";
    statusMessage.value = "Todo created.";
  } catch (error) {
    setError(error, "Unable to create todo.");
  } finally {
    savingTodo.value = false;
  }
}

function startEditing(todo: Todo) {
  editingTodoId.value = todo.id;
  editForm.title = todo.title;
  editForm.description = todo.description;
  clearMessages();
}

function cancelEditing() {
  editingTodoId.value = null;
  editForm.title = "";
  editForm.description = "";
}

async function saveTodo(todo: Todo) {
  clearMessages();

  const title = editForm.title.trim();
  const description = editForm.description.trim();

  if (!title) {
    errorMessage.value = "Todo title is required.";
    return;
  }

  try {
    const updatedTodo = await api.updateTodo(todo.id, {
      title,
      description,
    });
    replaceTodo(updatedTodo);
    cancelEditing();
    statusMessage.value = "Todo updated.";
  } catch (error) {
    setError(error, "Unable to update todo.");
  }
}

async function toggleTodo(todo: Todo) {
  clearMessages();

  try {
    const updatedTodo = await api.updateTodo(todo.id, {
      completed: !todo.completed,
    });
    replaceTodo(updatedTodo);
  } catch (error) {
    setError(error, "Unable to update todo.");
  }
}

async function deleteTodo(todo: Todo) {
  clearMessages();

  try {
    await api.deleteTodo(todo.id);
    todos.value = todos.value.filter((item) => item.id !== todo.id);
    statusMessage.value = "Todo deleted.";
  } catch (error) {
    setError(error, "Unable to delete todo.");
  }
}

function openOidcLogin() {
  window.location.href = oidcLoginUrl;
}

function isOwner(todo: Todo) {
  return currentUser.value?.id === todo.user_id;
}

function ownerName(todo: Todo) {
  return todo.user?.username ?? `User ${todo.user_id}`;
}

function replaceTodo(updatedTodo: Todo) {
  todos.value = todos.value.map((todo) => (todo.id === updatedTodo.id ? updatedTodo : todo));
}

function setSession(nextToken: string, user: User) {
  token.value = nextToken;
  currentUser.value = user;
  localStorage.setItem(TOKEN_KEY, nextToken);
  localStorage.setItem(USER_KEY, JSON.stringify(user));
}

function signOut() {
  clearSession();
  authMode.value = "login";
  statusMessage.value = "Signed out.";
}

function handleUnauthorized() {
  clearSession();
  authMode.value = "login";
  errorMessage.value = "Your session expired. Please sign in again.";
}

function clearSession() {
  token.value = null;
  currentUser.value = null;
  localStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem(USER_KEY);
}

function clearMessages() {
  errorMessage.value = "";
  statusMessage.value = "";
}

function setError(error: unknown, fallback: string) {
  if (error instanceof ApiError && error.status === 401 && errorMessage.value) {
    return;
  }

  errorMessage.value = error instanceof ApiError ? error.message : fallback;
}

function readStoredUser() {
  const rawUser = localStorage.getItem(USER_KEY);
  if (!rawUser) {
    return null;
  }

  try {
    return JSON.parse(rawUser) as User;
  } catch {
    localStorage.removeItem(USER_KEY);
    return null;
  }
}
</script>

<template>
  <main class="min-h-screen">
    <div class="mx-auto flex w-full max-w-6xl flex-col gap-6 px-4 py-6 sm:px-6 lg:px-8">
      <header class="flex flex-col gap-4 border-b border-border pb-5 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <p class="text-sm font-medium uppercase tracking-wide text-primary">SOC2 Todo Workspace</p>
          <h1 class="mt-1 text-3xl font-semibold text-foreground">Todo App</h1>
          <p class="mt-2 max-w-2xl text-sm text-muted-foreground">
            Track shared todos, sign in to create items, and manage only the todos you own.
          </p>
        </div>
        <div class="flex items-center gap-2">
          <Badge variant="secondary">{{ openCount }} open</Badge>
          <Badge variant="outline">{{ completedCount }} completed</Badge>
        </div>
      </header>

      <div
        v-if="errorMessage || statusMessage"
        class="rounded-md border px-4 py-3 text-sm"
        :class="
          errorMessage
            ? 'border-destructive/35 bg-destructive/10 text-destructive'
            : 'border-primary/25 bg-primary/10 text-primary'
        "
        :role="errorMessage ? 'alert' : 'status'"
        :aria-live="errorMessage ? 'assertive' : 'polite'"
      >
        {{ errorMessage || statusMessage }}
      </div>

      <div class="grid gap-6 lg:grid-cols-[320px_1fr]">
        <aside class="flex flex-col gap-6">
          <Card class="p-5">
            <template v-if="currentUser">
              <div class="flex items-start justify-between gap-4">
                <div>
                  <p class="text-sm text-muted-foreground">Signed in as</p>
                  <p class="mt-1 text-lg font-semibold">{{ currentUser.username }}</p>
                </div>
                <Button variant="outline" size="sm" @click="signOut">
                  <LogOut class="mr-2 h-4 w-4" />
                  Sign out
                </Button>
              </div>
            </template>

            <template v-else>
              <div class="mb-5 flex rounded-md border border-border bg-muted p-1">
                <button
                  class="h-9 flex-1 rounded-sm text-sm font-medium transition-colors"
                  :class="authMode === 'login' ? 'bg-card text-foreground shadow-sm' : 'text-muted-foreground'"
                  type="button"
                  @click="authMode = 'login'"
                >
                  Login
                </button>
                <button
                  class="h-9 flex-1 rounded-sm text-sm font-medium transition-colors"
                  :class="authMode === 'register' ? 'bg-card text-foreground shadow-sm' : 'text-muted-foreground'"
                  type="button"
                  @click="authMode = 'register'"
                >
                  Register
                </button>
              </div>

              <form class="space-y-4" @submit.prevent="submitAuth">
                <div class="space-y-2">
                  <Label for="username">Username</Label>
                  <Input id="username" v-model="authForm.username" required :minlength="3" :maxlength="255" />
                </div>
                <div class="space-y-2">
                  <Label for="password">Password</Label>
                  <Input id="password" v-model="authForm.password" type="password" required :minlength="6" />
                </div>
                <Button class="w-full" type="submit" :disabled="authLoading">
                  <component :is="authMode === 'register' ? UserPlus : LogIn" class="mr-2 h-4 w-4" />
                  {{ authLoading ? "Working..." : authMode === "register" ? "Create account" : "Sign in" }}
                </Button>
              </form>

              <Button v-if="oidcLoginUrl" class="mt-3 w-full" variant="outline" @click="openOidcLogin">
                <ShieldCheck class="mr-2 h-4 w-4" />
                Login with provider
              </Button>
            </template>
          </Card>

          <Card class="p-5">
            <h2 class="text-lg font-semibold">Create todo</h2>
            <form class="mt-4 space-y-4" @submit.prevent="createTodo">
              <div class="space-y-2">
                <Label for="todo-title">Title</Label>
                <Input
                  id="todo-title"
                  v-model="todoForm.title"
                  required
                  :maxlength="255"
                  :disabled="!currentUser || savingTodo"
                />
              </div>
              <div class="space-y-2">
                <Label for="todo-description">Description</Label>
                <Textarea
                  id="todo-description"
                  v-model="todoForm.description"
                  :maxlength="1000"
                  :disabled="!currentUser || savingTodo"
                />
              </div>
              <Button class="w-full" type="submit" :disabled="!currentUser || savingTodo">
                <Plus class="mr-2 h-4 w-4" />
                {{ savingTodo ? "Creating..." : "Create todo" }}
              </Button>
            </form>
          </Card>
        </aside>

        <section class="space-y-4">
          <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <h2 class="text-xl font-semibold">All todos</h2>
            <Button variant="outline" size="sm" :disabled="loadingTodos" @click="loadTodos">
              <RefreshCw class="mr-2 h-4 w-4" :class="{ 'animate-spin': loadingTodos }" />
              Refresh
            </Button>
          </div>

          <div v-if="loadingTodos" class="rounded-md border border-dashed border-border p-8 text-center text-muted-foreground">
            Loading todos...
          </div>

          <div v-else-if="todos.length === 0" class="rounded-md border border-dashed border-border p-8 text-center text-muted-foreground">
            No todos yet.
          </div>

          <div v-else class="grid gap-4">
            <Card v-for="todo in todos" :key="todo.id" class="p-5">
              <div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
                <div class="min-w-0 flex-1">
                  <div class="flex flex-wrap items-center gap-2">
                    <Badge :variant="todo.completed ? 'secondary' : 'outline'">
                      <Check v-if="todo.completed" class="mr-1 h-3.5 w-3.5" />
                      <Circle v-else class="mr-1 h-3.5 w-3.5" />
                      {{ todo.completed ? "Completed" : "Open" }}
                    </Badge>
                    <span class="text-sm text-muted-foreground">Owner: {{ ownerName(todo) }}</span>
                  </div>

                  <form v-if="editingTodoId === todo.id" class="mt-4 space-y-3" @submit.prevent="saveTodo(todo)">
                    <Input v-model="editForm.title" required :maxlength="255" />
                    <Textarea v-model="editForm.description" :maxlength="1000" />
                    <div class="flex flex-wrap gap-2">
                      <Button type="submit" size="sm">
                        <Save class="mr-2 h-4 w-4" />
                        Save
                      </Button>
                      <Button variant="outline" size="sm" @click="cancelEditing">
                        <X class="mr-2 h-4 w-4" />
                        Cancel
                      </Button>
                    </div>
                  </form>

                  <template v-else>
                    <h3 class="mt-3 break-words text-lg font-semibold" :class="{ 'text-muted-foreground line-through': todo.completed }">
                      {{ todo.title }}
                    </h3>
                    <p v-if="todo.description" class="mt-2 whitespace-pre-wrap break-words text-sm text-muted-foreground">
                      {{ todo.description }}
                    </p>
                    <p v-else class="mt-2 text-sm italic text-muted-foreground">No description.</p>
                  </template>
                </div>

                <div v-if="isOwner(todo)" class="flex shrink-0 gap-2">
                  <Button
                    variant="outline"
                    size="icon"
                    :aria-label="todo.completed ? 'Mark open' : 'Mark completed'"
                    :title="todo.completed ? 'Mark open' : 'Mark completed'"
                    @click="toggleTodo(todo)"
                  >
                    <Check class="h-4 w-4" />
                  </Button>
                  <Button variant="outline" size="icon" aria-label="Edit todo" title="Edit todo" @click="startEditing(todo)">
                    <Pencil class="h-4 w-4" />
                  </Button>
                  <Button variant="destructive" size="icon" aria-label="Delete todo" title="Delete todo" @click="deleteTodo(todo)">
                    <Trash2 class="h-4 w-4" />
                  </Button>
                </div>
              </div>
            </Card>
          </div>
        </section>
      </div>
    </div>
  </main>
</template>
