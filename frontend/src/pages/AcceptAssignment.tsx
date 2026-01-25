import { type Component, createResource, createSignal, Show } from 'solid-js';
import { useParams, useNavigate } from '@solidjs/router';
import { assignmentAPI, submissionAPI, authAPI } from '../services/api';
import { authStore } from '../stores/auth';

const AcceptAssignment: Component = () => {
  const params = useParams();
  const navigate = useNavigate();
  const [accepting, setAccepting] = createSignal(false);
  const [error, setError] = createSignal('');

  const [assignment] = createResource(
    () => params.id ? parseInt(params.id) : 0,
    async (id) => {
      if (!id) return null;
      const response = await assignmentAPI.get(id);
      return response.data;
    }
  );

  const isLoggedIn = () => !!authStore.user();
  const isAuthLoading = () => authStore.loading();

  const handleLogin = async () => {
    localStorage.setItem('redirect_after_login', `/accept/${params.id}`);
    const response = await authAPI.login();
    window.location.href = response.data.url;
  };

  const handleAccept = async () => {
    if (!isLoggedIn()) {
      handleLogin();
      return;
    }

    setAccepting(true);
    setError('');

    try {
      const assignmentId = parseInt(params.id || '0');
      const response = await submissionAPI.accept(assignmentId);
      window.open(response.data.repo_url, '_blank');
      const a = assignment();
      if (a?.course?.slug) {
        navigate(`/courses/${a.course.slug}`);
      } else {
        navigate('/dashboard');
      }
    } catch (err: any) {
      if (err.response?.status === 401) {
        handleLogin();
      } else if (err.response?.status === 403) {
        setError('You need to enroll in the course first');
      } else {
        setError(err.response?.data?.message || 'Failed to accept assignment');
      }
    } finally {
      setAccepting(false);
    }
  };

  const formatDeadline = (deadline: string) => {
    return new Date(deadline).toLocaleDateString('ru-RU', {
      year: 'numeric',
      month: 'long',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  return (
    <div class="container mx-auto px-4 py-16">
      <div class="max-w-md mx-auto">
        <Show
          when={!assignment.loading}
          fallback={
            <div class="text-center py-8">
              <div class="text-xl">Loading assignment...</div>
            </div>
          }
        >
          <Show
            when={assignment()}
            fallback={
              <div class="text-center py-8">
                <div class="text-xl text-red-600 mb-4">Assignment not found</div>
              </div>
            }
          >
            {(a) => (
              <div class="bg-white rounded-lg shadow-lg p-8">
                <h1 class="text-2xl font-bold mb-2">Accept Assignment</h1>
                <div class="border-b pb-4 mb-4">
                  <h2 class="text-xl font-semibold text-blue-600">{a().title}</h2>
                  <p class="text-gray-600 mt-2">{a().description}</p>
                </div>

                <div class="mb-6 text-sm text-gray-500 space-y-1">
                  <div>Deadline: {formatDeadline(a().deadline)}</div>
                  <div>Max Points: {a().max_points}</div>
                  {a().template_repo && (
                    <div>Template: {a().template_repo}</div>
                  )}
                </div>

                <Show when={error()}>
                  <div class="mb-4 p-3 bg-red-100 text-red-700 rounded">
                    {error()}
                  </div>
                </Show>

                <button
                  onClick={handleAccept}
                  disabled={accepting() || isAuthLoading()}
                  class="w-full bg-green-600 hover:bg-green-700 disabled:bg-green-400 text-white py-3 rounded font-medium"
                >
                  {isAuthLoading()
                    ? 'Loading...'
                    : accepting()
                    ? 'Creating repository...'
                    : isLoggedIn()
                    ? 'Accept Assignment'
                    : 'Login to Accept Assignment'}
                </button>

                <p class="mt-4 text-sm text-gray-500 text-center">
                  {isLoggedIn()
                    ? 'A personal repository will be created for you in the course organization.'
                    : 'You need to login and be enrolled in the course to accept this assignment.'}
                </p>
              </div>
            )}
          </Show>
        </Show>
      </div>
    </div>
  );
};

export default AcceptAssignment;
