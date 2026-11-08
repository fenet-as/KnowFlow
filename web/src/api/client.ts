// KnowFlow Typed API Client

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
  getToken?: () => Promise<string | null>;
}

export class ApiClient {
  private baseUrl: string;
  private getToken?: () => Promise<string | null>;

  constructor(options: ApiClientOptions = {}) {
    const defaultUrl = import.meta.env.VITE_API_BASE_URL || '/api/v1';
    this.baseUrl = (options.baseUrl || defaultUrl).replace(/\/$/, '');
    this.getToken = options.getToken;
  }

  private async request<T>(path: string, options: RequestInit = {}): Promise<T> {
    const url = `${this.baseUrl}${path.startsWith('/') ? path : `/${path}`}`;
    const headers = new Headers(options.headers || {});

    // Attach Clerk JWT Bearer token if token provider is configured
    if (this.getToken) {
      try {
        const token = await this.getToken();
        if (token) {
          headers.set('Authorization', `Bearer ${token}`);
        }
      } catch (err) {
        console.warn('Failed to acquire auth token for request', err);
      }
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
      headers,
    });

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
        // Response is non-JSON
      }

      throw new ApiError(response.status, errPayload);
    }

    if (response.status === 204) {
      return undefined as unknown as T;
    }

    return (await response.json()) as T;
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

// React context or hook helper for convenient usage
export function createApiClient(getToken?: () => Promise<string | null>): ApiClient {
  return new ApiClient({ getToken });
}

