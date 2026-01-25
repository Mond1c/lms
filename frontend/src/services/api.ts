import axios from 'axios';

const API_URL = 'http://localhost:8080/api';

export const api = axios.create({
  baseURL: API_URL,
  headers: {
    'Content-Type': 'application/json',
  },
});

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

export interface User {
  id: number;
  username: string;
  email: string;
  full_name: string;
  avatar_url: string;
  is_admin: boolean;
}

export interface Course {
  id: number;
  name: string;
  description: string;
  slug: string;
  org_name: string;
  invite_code: string;
  created_at: string;
  is_instructor?: boolean;
  instructors?: User[];
  assignments?: Assignment[];
  students?: Student[];
}

export interface Assignment {
  id: number;
  course_id: number;
  title: string;
  description: string;
  template_repo: string;
  deadline: string;
  max_points: number;
  submissions?: Submission[];
  course?: Course;
}

export interface Student {
  id: number;
  course_id: number;
  gitea_id: number;
  username: string;
  email: string;
  full_name: string;
}

export interface Submission {
  id: number;
  assignment_id: number;
  student_id: number;
  repo_url: string;
  status: string;
  score: number | null;
  feedback: string;
  submitted_at: string | null;
  student?: Student;
  assignment?: Assignment;
}

export const authAPI = {
  login: () => api.get<{ url: string }>('/auth/login'),
  me: () => api.get<User>('/auth/me'),
};

export const courseAPI = {
  list: () => api.get<Course[]>('/courses'),
  listEnrolled: () => api.get<Course[]>('/courses/enrolled'),
  get: (slug: string) => api.get<Course>(`/courses/${slug}`),
  create: (data: { name: string; description: string }) =>
    api.post<Course>('/courses', data),
  getByInviteCode: (code: string) => api.get<Course>(`/invite/${code}`),
  regenerateInviteCode: (slug: string) =>
    api.post<{ invite_code: string }>(`/courses/${slug}/regenerate-invite`),
};

export const assignmentAPI = {
  list: (courseSlug: string) =>
    api.get<Assignment[]>(`/courses/${courseSlug}/assignments`),
  get: (id: number) => api.get<Assignment>(`/assignments/${id}`),
  create: (
    courseSlug: string,
    data: {
      title: string;
      description: string;
      template_repo: string;
      deadline: string;
      max_points: number;
    }
  ) => api.post<Assignment>(`/courses/${courseSlug}/assignments`, data),
  update: (
    id: number,
    data: Partial<{
      title: string;
      description: string;
      template_repo: string;
      deadline: string;
      max_points: number;
    }>
  ) => api.put<Assignment>(`/assignments/${id}`, data),
  delete: (id: number) => api.delete(`/assignments/${id}`),
};

export const studentAPI = {
  list: (courseSlug: string) =>
    api.get<Student[]>(`/courses/${courseSlug}/students`),
  get: (id: number) => api.get<Student>(`/students/${id}`),
  enroll: (courseSlug: string) =>
    api.post<Student>(`/courses/${courseSlug}/enroll`),
  remove: (id: number) => api.delete(`/students/${id}`),
};

export const submissionAPI = {
  accept: (assignmentId: number) =>
    api.post<Submission>(`/assignments/${assignmentId}/accept`),
  list: (assignmentId: number) =>
    api.get<Submission[]>(`/assignments/${assignmentId}/submissions`),
  get: (id: number) => api.get<Submission>(`/submissions/${id}`),
  grade: (id: number, data: { score: number; feedback: string }) =>
    api.post<Submission>(`/submissions/${id}/grade`, data),
};