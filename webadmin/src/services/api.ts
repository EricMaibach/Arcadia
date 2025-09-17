import axios from 'axios';

const API_BASE_URL = 'http://localhost:8080';

const api = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json',
  },
});

// Types based on the Go structs
export interface ToolInfo {
  name: string;
  inputFormat: string;
}

export interface App {
  appId: string;
  version: string;
  runtime: string;
  tools: ToolInfo[];
  artifactUri: string;
  sourceLanguage?: string;
  files?: Array<{
    name: string;
    content: string;
  }>;
}

export interface RunToolRequest {
  appId: string;
  toolName: string;
  input: any;
}

export interface AppRequest {
  appId: string;
  version: string;
  runtime: string;
  tools: ToolInfo[];
  appSrc: string;
  dependencies?: { [key: string]: string };
}

export interface RecurrenceRule {
  interval: number;
  unit: string;
  daysOfWeek?: number[];
  endDate?: string;
}

export interface AppSchedule {
  id: string;
  appId: string;
  toolName: string;
  input: any;
  scheduleType: 'one-time' | 'recurring';
  scheduledTime: string;
  recurrence?: RecurrenceRule;
  isActive: boolean;
  createdAt: string;
  lastRun?: string;
  nextRun?: string;
  runCount: number;
}

export interface ScheduleRequest {
  appId: string;
  toolName: string;
  input: any;
  scheduleType: 'one-time' | 'recurring';
  scheduledTime: string;
  recurrence?: RecurrenceRule;
}

export interface ScheduledRun {
  id: string;
  scheduleId: string;
  appId: string;
  toolName: string;
  input: any;
  startedAt: string;
  completedAt?: string;
  status: 'running' | 'completed' | 'failed';
  output?: string;
  error?: string;
}

// AI Service Types
export interface AIProviderInfo {
  name: string;
  model: string;
  features: string[];
}

export interface AIProviderStatusResponse {
  current_provider: AIProviderInfo;
}

export interface AIChatRequest {
  message: string;
  context_id: string;
}

export interface AIChatResponse {
  response: string;
  context_stats?: {
    message_count: number;
    total_tokens: number;
  };
}

// File Watcher Types
export interface AddWatchDirRequest {
  path: string;
}

export interface RemoveWatchDirRequest {
  path: string;
}

export interface WatchedDirResponse {
  directories: string[];
}

// App Management API
export const appApi = {
  listApps: async (): Promise<App[]> => {
    const response = await api.get('/list_apps');
    return response.data;
  },

  runTool: async (request: RunToolRequest): Promise<any> => {
    const response = await api.post('/run_tool', request);
    return response.data;
  },

  submitAppSrc: async (request: AppRequest): Promise<any> => {
    const response = await api.post('/submit_app_src', request);
    return response.data;
  },
};

// Schedule Management API
export const scheduleApi = {
  listSchedules: async (appId?: string): Promise<AppSchedule[]> => {
    const params = appId ? { appId } : {};
    const response = await api.get('/list_schedules', { params });
    return response.data;
  },

  getSchedule: async (id: string): Promise<AppSchedule> => {
    const response = await api.get('/get_schedule', { params: { id } });
    return response.data;
  },

  createSchedule: async (request: ScheduleRequest): Promise<any> => {
    const response = await api.post('/schedule_app_run', request);
    return response.data;
  },

  updateSchedule: async (id: string, updates: Partial<AppSchedule>): Promise<any> => {
    const response = await api.put(`/update_schedule?id=${id}`, updates);
    return response.data;
  },

  deleteSchedule: async (id: string): Promise<any> => {
    const response = await api.delete(`/delete_schedule?id=${id}`);
    return response.data;
  },

  listScheduledRuns: async (scheduleId?: string): Promise<ScheduledRun[]> => {
    const params = scheduleId ? { schedule_id: scheduleId } : {};
    const response = await api.get('/list_scheduled_runs', { params });
    return response.data;
  },
};

// AI Service API
export const aiApi = {
  getProviderStatus: async (): Promise<AIProviderInfo> => {
    const response = await api.get<AIProviderStatusResponse>('/api/ai/provider/status');
    return response.data.current_provider;
  },

  sendMessage: async (request: AIChatRequest): Promise<AIChatResponse> => {
    const response = await api.post<AIChatResponse>('/api/ai/v2/chat', request);
    return response.data;
  },
};

// File Watcher API
export const fileWatcherApi = {
  listWatchedDirectories: async (): Promise<string[]> => {
    const response = await api.get<WatchedDirResponse>('/filewatcher/list');
    return response.data.directories;
  },

  addWatchedDirectory: async (path: string): Promise<any> => {
    const request: AddWatchDirRequest = { path };
    const response = await api.post('/filewatcher/add', request);
    return response.data;
  },

  removeWatchedDirectory: async (path: string): Promise<any> => {
    const request: RemoveWatchDirRequest = { path };
    const response = await api.post('/filewatcher/remove', request);
    return response.data;
  },
};

export default api;