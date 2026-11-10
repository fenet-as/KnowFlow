// KnowFlow Typed API Client

export interface User {
  id: string;
  email: string;
  full_name: string;
  role: string;
  created_at: string;
}

export interface AuthResponse {
  user: User;
  access_token: string;
  expires_in: number;
}

export interface DocumentItem {
  id: string;
  filename: string;
  size_bytes: number;
  status: 'queued' | 'processing' | 'ready' | 'failed';
  error_message: string | null;
  chunk_count: number;
  created_at: string;
  updated_at: string;
}

export interface DocumentListResponse {
  items: DocumentItem[];
  next_cursor: string | null;
}

export interface ChunkDetail {
  id: string;
  document_id: string;
  document_name: string;
  chunk_index: number;
  heading: string | null;
  content: string;
  token_count: number;
  created_at: string;
}

export interface Citation {
  marker: number;
  chunk_id: string;
  document_id: string;
  document_name: string;
  heading: string | null;
  snippet: string;
}

export interface AskResponse {
  answer: string;
  grounded: boolean;
  citations: Citation[];
}

export interface Message {
  id: string;
  conversation_id: string;
  role: 'user' | 'assistant';
  content: string;
  citations: Citation[];
  created_at: string;
}

export interface Conversation {
  id: string;
  title: string;
  created_at: string;
  updated_at: string;
}

export interface ConversationDetail extends Conversation {
  messages: Message[];
}

export interface ConversationListResponse {
  items: Conversation[];
  next_cursor: string | null;
}

export interface MessageListResponse {
  items: Message[];
  next_cursor: string | null;
}

export interface ConversationTurnResponse {
  user_message: Message;
  assistant_message: Message;
}

export interface ApiErrorDetail {
  code: string;
  message: string;
  request_id?: string;
  details?: unknown;
}

export class ApiError extends Error {
  status: number;
  code: string;
  requestId?: string;
  details?: unknown;

  constructor(status: number, errorDetail: ApiErrorDetail) {
    super(errorDetail.message || `API error with status ${status}`);
    this.name = 'ApiError';
    this.status = status;
    this.code = errorDetail.code || 'UNKNOWN_ERROR';
    this.requestId = errorDetail.request_id;
    this.details = errorDetail.details;
  }
}

export interface ApiClientOptions {
  baseUrl?: string;
  initialToken?: string | null;
  onTokenRefreshed?: (token: string | null) => void;
  onUnauthorized?: () => void;
}

export class ApiClient {
  private baseUrl: string;
  private accessToken: string | null = null;
  private refreshPromise: Promise<string | null> | null = null;
  private onTokenRefreshed?: (token: string | null) => void;
  private onUnauthorized?: () => void;

  constructor(options: ApiClientOptions = {}) {
    const defaultUrl = import.meta.env.VITE_API_BASE_URL || '/api/v1';
    this.baseUrl = (options.baseUrl || defaultUrl).replace(/\/$/, '');
    this.accessToken = options.initialToken || null;
    this.onTokenRefreshed = options.onTokenRefreshed;
    this.onUnauthorized = options.onUnauthorized;
  }

  setAccessToken(token: string | null) {
    this.accessToken = token;
  }

  getAccessToken(): string | null {
    return this.accessToken;
  }

  private async refreshAccessToken(): Promise<string | null> {
    if (this.refreshPromise) {
      return this.refreshPromise;
    }

    this.refreshPromise = (async () => {
      try {
        const url = `${this.baseUrl}/auth/refresh`;
        const res = await fetch(url, {
          method: 'POST',
          credentials: 'include',
        });
        if (!res.ok) {
          this.accessToken = null;
          this.onTokenRefreshed?.(null);
          return null;
        }
        const data = (await res.json()) as AuthResponse;
        this.accessToken = data.access_token;
        this.onTokenRefreshed?.(data.access_token);
        return data.access_token;
      } catch (err) {
        this.accessToken = null;
        this.onTokenRefreshed?.(null);
        return null;
      } finally {
        this.refreshPromise = null;
      }
    })();

    return this.refreshPromise;
  }

  private async request<T>(path: string, options: RequestInit = {}, retryOn401 = true): Promise<T> {
    const url = `${this.baseUrl}${path.startsWith('/') ? path : `/${path}`}`;
    const headers = new Headers(options.headers || {});

    if (this.accessToken) {
      headers.set('Authorization', `Bearer ${this.accessToken}`);
    }

    // Set default headers for JSON payloads
    if (
      !(options.body instanceof FormData) &&
      !headers.has('Content-Type') &&
      options.method &&
      options.method !== 'GET' &&
      options.method !== 'HEAD'
    ) {
      headers.set('Content-Type', 'application/json');
    }

    const response = await fetch(url, {
      ...options,
      credentials: 'include',
      headers,
    });

    if (response.status === 401 && retryOn401 && !path.startsWith('/auth/')) {
      const newToken = await this.refreshAccessToken();
      if (newToken) {
        return this.request<T>(path, options, false);
      }
      this.onUnauthorized?.();
    }

    if (!response.ok) {
      let errPayload: ApiErrorDetail = {
        code: 'HTTP_ERROR',
        message: response.statusText,
      };

      try {
        const json = await response.json();
        if (json && json.error) {
          errPayload = json.error;
        }
      } catch {
        // Non-JSON response
      }

      throw new ApiError(response.status, errPayload);
    }

    if (response.status === 204) {
      return undefined as unknown as T;
    }

    return (await response.json()) as T;
  }

  // --- Native Auth Endpoints ---

  async register(req: { email: string; password: string; full_name: string }): Promise<AuthResponse> {
    const resp = await this.request<AuthResponse>('/auth/register', {
      method: 'POST',
      body: JSON.stringify(req),
    }, false);
    this.setAccessToken(resp.access_token);
    this.onTokenRefreshed?.(resp.access_token);
    return resp;
  }

  async login(req: { email: string; password: string }): Promise<AuthResponse> {
    const resp = await this.request<AuthResponse>('/auth/login', {
      method: 'POST',
      body: JSON.stringify(req),
    }, false);
    this.setAccessToken(resp.access_token);
    this.onTokenRefreshed?.(resp.access_token);
    return resp;
  }

  async refresh(): Promise<AuthResponse> {
    const resp = await this.request<AuthResponse>('/auth/refresh', {
      method: 'POST',
    }, false);
    this.setAccessToken(resp.access_token);
    this.onTokenRefreshed?.(resp.access_token);
    return resp;
  }

  async logout(): Promise<void> {
    try {
      await this.request<void>('/auth/logout', {
        method: 'POST',
      }, false);
    } finally {
      this.setAccessToken(null);
      this.onTokenRefreshed?.(null);
    }
  }

  async getMe(): Promise<User> {
    return this.request<User>('/auth/me', {
      method: 'GET',
    });
  }

  async updateProfile(req: { full_name: string; email: string }): Promise<User> {
    return this.request<User>('/auth/profile', {
      method: 'PATCH',
      body: JSON.stringify(req),
    });
  }

  async changePassword(req: { current_password: string; new_password: string }): Promise<void> {
    await this.request<{ status: string }>('/auth/change-password', {
      method: 'POST',
      body: JSON.stringify(req),
    });
  }

  // --- Document Endpoints (KF-13, KF-14) ---

  async listDocuments(params?: { limit?: number; cursor?: string }): Promise<DocumentListResponse> {
    const query = new URLSearchParams();
    if (params?.limit) query.set('limit', params.limit.toString());
    if (params?.cursor) query.set('cursor', params.cursor);
    const qs = query.toString();
    return this.request<DocumentListResponse>(`/documents${qs ? `?${qs}` : ''}`, {
      method: 'GET',
    });
  }

  async getDocument(id: string): Promise<DocumentItem> {
    return this.request<DocumentItem>(`/documents/${id}`, {
      method: 'GET',
    });
  }

  async uploadDocument(file: File): Promise<DocumentItem> {
    const formData = new FormData();
    formData.append('file', file);
    return this.request<DocumentItem>('/documents', {
      method: 'POST',
      body: formData,
    });
  }

  async deleteDocument(id: string): Promise<void> {
    return this.request<void>(`/documents/${id}`, {
      method: 'DELETE',
    });
  }

  // --- Chunk Endpoints (KF-21) ---

  async getChunk(id: string): Promise<ChunkDetail> {
    return this.request<ChunkDetail>(`/chunks/${id}`, {
      method: 'GET',
    });
  }

  // --- Stateless Ask (KF-20) ---

  async askQuestion(question: string, documentIDs?: string[]): Promise<AskResponse> {
    return this.request<AskResponse>('/ask', {
      method: 'POST',
      body: JSON.stringify({
        question,
        document_ids: documentIDs && documentIDs.length > 0 ? documentIDs : undefined,
      }),
    });
  }

  // --- Conversations and Messages (KF-28, KF-29) ---

  async createConversation(title?: string): Promise<Conversation> {
    return this.request<Conversation>('/conversations', {
      method: 'POST',
      body: JSON.stringify({ title }),
    });
  }

  async listConversations(params?: { limit?: number; cursor?: string }): Promise<ConversationListResponse> {
    const query = new URLSearchParams();
    if (params?.limit) query.set('limit', params.limit.toString());
    if (params?.cursor) query.set('cursor', params.cursor);
    const qs = query.toString();
    return this.request<ConversationListResponse>(`/conversations${qs ? `?${qs}` : ''}`, {
      method: 'GET',
    });
  }

  async getConversation(id: string, limit?: number): Promise<ConversationDetail> {
    const qs = limit ? `?limit=${limit}` : '';
    return this.request<ConversationDetail>(`/conversations/${id}${qs}`, {
      method: 'GET',
    });
  }

  async deleteConversation(id: string): Promise<void> {
    return this.request<void>(`/conversations/${id}`, {
      method: 'DELETE',
    });
  }

  async listMessages(convId: string, params?: { limit?: number; cursor?: string }): Promise<MessageListResponse> {
    const query = new URLSearchParams();
    if (params?.limit) query.set('limit', params.limit.toString());
    if (params?.cursor) query.set('cursor', params.cursor);
    const qs = query.toString();
    return this.request<MessageListResponse>(`/conversations/${convId}/messages${qs ? `?${qs}` : ''}`, {
      method: 'GET',
    });
  }

  async postConversationMessage(
    convId: string,
    content: string,
    documentIDs?: string[],
  ): Promise<ConversationTurnResponse> {
    return this.request<ConversationTurnResponse>(`/conversations/${convId}/messages`, {
      method: 'POST',
      body: JSON.stringify({
        content,
        document_ids: documentIDs && documentIDs.length > 0 ? documentIDs : undefined,
      }),
    });
  }
}

// Global shared client singleton
export const apiClient = new ApiClient();
