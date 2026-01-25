import { type Component, createResource, createSignal, Show } from 'solid-js';
import { useParams, useNavigate } from '@solidjs/router';
import { courseAPI, studentAPI, authAPI } from '../services/api';
import { authStore } from '../stores/auth';

const JoinCourse: Component = () => {
  const params = useParams();
  const navigate = useNavigate();
  const [enrolling, setEnrolling] = createSignal(false);
  const [error, setError] = createSignal('');

  const [course] = createResource(
    () => params.code,
    async (code) => {
      if (!code) return null;
      const response = await courseAPI.getByInviteCode(code);
      return response.data;
    }
  );

  const isLoggedIn = () => !!authStore.user();
  const isAuthLoading = () => authStore.loading();

  const handleLogin = async () => {
    localStorage.setItem('redirect_after_login', `/join/${params.code}`);
    const response = await authAPI.login();
    window.location.href = response.data.url;
  };

  const handleEnroll = async () => {
    const c = course();
    if (!c) return;

    if (!isLoggedIn()) {
      handleLogin();
      return;
    }

    setEnrolling(true);
    setError('');

    try {
      await studentAPI.enroll(c.slug);
      navigate(`/courses/${c.slug}`);
    } catch (err: any) {
      if (err.response?.status === 409) {
        navigate(`/courses/${c.slug}`);
      } else if (err.response?.status === 401) {
        handleLogin();
      } else {
        setError(err.response?.data?.message || 'Failed to enroll');
      }
    } finally {
      setEnrolling(false);
    }
  };

  return (
    <div class="container mx-auto px-4 py-16">
      <div class="max-w-md mx-auto">
        <Show
          when={!course.loading}
          fallback={
            <div class="text-center py-8">
              <div class="text-xl">Loading course...</div>
            </div>
          }
        >
          <Show
            when={course()}
            fallback={
              <div class="text-center py-8">
                <div class="text-xl text-red-600 mb-4">Invalid invite link</div>
                <p class="text-gray-600">
                  This invite link is invalid or has expired.
                </p>
              </div>
            }
          >
            {(c) => (
              <div class="bg-white rounded-lg shadow-lg p-8">
                <h1 class="text-2xl font-bold mb-2">Join Course</h1>
                <div class="border-b pb-4 mb-4">
                  <h2 class="text-xl font-semibold text-blue-600">{c().name}</h2>
                  <p class="text-gray-600 mt-2">{c().description}</p>
                </div>

                <div class="mb-6 text-sm text-gray-500">
                  <div>Organization: {c().org_name}</div>
                </div>

                <Show when={error()}>
                  <div class="mb-4 p-3 bg-red-100 text-red-700 rounded">
                    {error()}
                  </div>
                </Show>

                <button
                  onClick={handleEnroll}
                  disabled={enrolling() || isAuthLoading()}
                  class="w-full bg-blue-600 hover:bg-blue-700 disabled:bg-blue-400 text-white py-3 rounded font-medium"
                >
                  {isAuthLoading()
                    ? 'Loading...'
                    : enrolling()
                    ? 'Enrolling...'
                    : isLoggedIn()
                    ? 'Enroll in Course'
                    : 'Login with Gitea to Enroll'}
                </button>

                <p class="mt-4 text-sm text-gray-500 text-center">
                  {isLoggedIn()
                    ? 'By enrolling, you will get access to course assignments and materials.'
                    : 'You need to login with your Gitea account to enroll in this course.'}
                </p>
              </div>
            )}
          </Show>
        </Show>
      </div>
    </div>
  );
};

export default JoinCourse;
