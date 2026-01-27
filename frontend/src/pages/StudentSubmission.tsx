import { type Component, createResource, createSignal, Show, onCleanup } from 'solid-js';
import { useParams, A } from '@solidjs/router';
import { submissionAPI, reviewAPI, type ReviewStatusResponse } from '../services/api';

const StudentSubmission: Component = () => {
  const params = useParams();
  const [requesting, setRequesting] = createSignal(false);
  const [cancelling, setCancelling] = createSignal(false);
  const [error, setError] = createSignal('');
  const [secondsRemaining, setSecondsRemaining] = createSignal(0);

  const [submission, { refetch: refetchSubmission }] = createResource(
    () => params.id ? parseInt(params.id) : 0,
    async (id) => {
      if (!id) return null;
      const response = await submissionAPI.get(id);
      return response.data;
    }
  );

  const [reviewStatus, { refetch: refetchReviewStatus }] = createResource(
    () => params.id ? parseInt(params.id) : 0,
    async (id) => {
      if (!id) return null;
      const response = await reviewAPI.getStatus(id);
      return response.data;
    }
  );

  // Timer for countdown
  let timerInterval: number | undefined;

  const startCountdown = (seconds: number) => {
    setSecondsRemaining(seconds);
    if (timerInterval) clearInterval(timerInterval);

    timerInterval = setInterval(() => {
      setSecondsRemaining(prev => {
        if (prev <= 1) {
          clearInterval(timerInterval);
          refetchReviewStatus();
          return 0;
        }
        return prev - 1;
      });
    }, 1000) as unknown as number;
  };

  onCleanup(() => {
    if (timerInterval) clearInterval(timerInterval);
  });

  // Start countdown when we get review status
  const updateCountdown = (status: ReviewStatusResponse | null | undefined) => {
    if (status?.has_active_request && status.seconds_remaining && status.seconds_remaining > 0) {
      startCountdown(status.seconds_remaining);
    }
  };

  // Watch for reviewStatus changes
  createResource(
    () => reviewStatus(),
    async (status) => {
      updateCountdown(status);
      return status;
    }
  );

  const handleRequestReview = async () => {
    setRequesting(true);
    setError('');

    try {
      const response = await reviewAPI.requestReview(parseInt(params.id || '0'));
      startCountdown(response.data.seconds_to_cancel);
      refetchReviewStatus();
    } catch (err: any) {
      setError(err.response?.data?.message || 'Failed to request review');
    } finally {
      setRequesting(false);
    }
  };

  const handleCancelReview = async () => {
    const status = reviewStatus();
    if (!status?.review_request?.id) return;

    setCancelling(true);
    setError('');

    try {
      await reviewAPI.cancelReview(status.review_request.id);
      if (timerInterval) clearInterval(timerInterval);
      setSecondsRemaining(0);
      refetchReviewStatus();
    } catch (err: any) {
      setError(err.response?.data?.message || 'Failed to cancel review');
    } finally {
      setCancelling(false);
    }
  };

  const formatTime = (seconds: number) => {
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${mins}:${secs.toString().padStart(2, '0')}`;
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'pending':
        return 'bg-yellow-100 text-yellow-800';
      case 'submitted':
        return 'bg-blue-100 text-blue-800';
      case 'reviewed':
        return 'bg-green-100 text-green-800';
      case 'cancelled':
        return 'bg-gray-100 text-gray-800';
      default:
        return 'bg-gray-100 text-gray-800';
    }
  };

  const getStatusText = (status: string) => {
    switch (status) {
      case 'pending':
        return 'Pending (can cancel)';
      case 'submitted':
        return 'Waiting for Review';
      case 'reviewed':
        return 'Reviewed';
      case 'cancelled':
        return 'Cancelled';
      default:
        return status;
    }
  };

  return (
    <div class="container mx-auto px-4 py-8">
      <Show
        when={!submission.loading && submission()}
        fallback={<div class="text-center py-8">Loading...</div>}
      >
        {(sub) => (
          <div class="max-w-2xl mx-auto">
            <A
              href={sub().assignment?.course_id ? `/courses/${sub().assignment?.course_id}` : '/dashboard'}
              class="text-blue-600 hover:underline mb-4 inline-block"
            >
              Back to Course
            </A>

            <div class="bg-white rounded-lg shadow-lg p-6">
              <h1 class="text-2xl font-bold mb-2">{sub().assignment?.title}</h1>
              <p class="text-gray-600 mb-4">{sub().assignment?.description}</p>

              <div class="border-t pt-4 mt-4">
                <h2 class="text-lg font-semibold mb-3">Your Submission</h2>

                <div class="space-y-2 text-sm">
                  <div>
                    <span class="text-gray-500">Repository: </span>
                    <a
                      href={sub().repo_url}
                      target="_blank"
                      rel="noopener noreferrer"
                      class="text-blue-600 hover:underline"
                    >
                      {sub().repo_url}
                    </a>
                  </div>
                  <div>
                    <span class="text-gray-500">Status: </span>
                    <span class={`px-2 py-1 rounded text-xs font-medium ${
                      sub().status === 'graded' ? 'bg-green-100 text-green-800' : 'bg-yellow-100 text-yellow-800'
                    }`}>
                      {sub().status}
                    </span>
                  </div>
                  {sub().score !== null && (
                    <div>
                      <span class="text-gray-500">Score: </span>
                      <span class="font-medium">{sub().score}/{sub().assignment?.max_points}</span>
                    </div>
                  )}
                  {sub().feedback && (
                    <div>
                      <span class="text-gray-500">Feedback: </span>
                      <span>{sub().feedback}</span>
                    </div>
                  )}
                </div>
              </div>

              <div class="border-t pt-4 mt-4">
                <h2 class="text-lg font-semibold mb-3">Review Request</h2>

                <Show when={error()}>
                  <div class="mb-4 p-3 bg-red-100 text-red-700 rounded text-sm">
                    {error()}
                  </div>
                </Show>

                <Show
                  when={!reviewStatus.loading}
                  fallback={<div class="text-gray-500">Loading review status...</div>}
                >
                  <Show
                    when={reviewStatus()?.has_active_request}
                    fallback={
                      <div>
                        <p class="text-gray-600 mb-4">
                          Ready to submit for review? Your code will be locked for pushes until reviewed.
                        </p>
                        <button
                          onClick={handleRequestReview}
                          disabled={requesting()}
                          class="bg-blue-600 hover:bg-blue-700 disabled:bg-blue-400 text-white px-6 py-2 rounded font-medium"
                        >
                          {requesting() ? 'Requesting...' : 'Request Review'}
                        </button>
                      </div>
                    }
                  >
                    {() => {
                      const status = reviewStatus()!;
                      const request = status.review_request!;

                      return (
                        <div>
                          <div class="mb-4">
                            <span class="text-gray-500">Review Status: </span>
                            <span class={`px-2 py-1 rounded text-xs font-medium ${getStatusColor(request.status)}`}>
                              {getStatusText(request.status)}
                            </span>
                          </div>

                          <Show when={request.status === 'pending'}>
                            <div class="bg-yellow-50 border border-yellow-200 rounded p-4 mb-4">
                              <p class="text-yellow-800 mb-2">
                                Your review request is pending. You can cancel within the next:
                              </p>
                              <div class="text-3xl font-bold text-yellow-600 mb-3">
                                {formatTime(secondsRemaining())}
                              </div>
                              <button
                                onClick={handleCancelReview}
                                disabled={cancelling() || secondsRemaining() <= 0}
                                class="bg-red-600 hover:bg-red-700 disabled:bg-red-400 text-white px-4 py-2 rounded"
                              >
                                {cancelling() ? 'Cancelling...' : 'Cancel Request'}
                              </button>
                            </div>
                            <p class="text-sm text-gray-500">
                              After the timer expires, your submission will be sent for review and you won't be able to push changes until reviewed.
                            </p>
                          </Show>

                          <Show when={request.status === 'submitted'}>
                            <div class="bg-blue-50 border border-blue-200 rounded p-4">
                              <p class="text-blue-800">
                                Your submission is waiting for review. Push access is locked until an instructor reviews your code.
                              </p>
                            </div>
                          </Show>

                          <Show when={request.status === 'reviewed'}>
                            <div class="bg-green-50 border border-green-200 rounded p-4">
                              <p class="text-green-800 mb-2">
                                Your code has been reviewed! Push access has been restored.
                              </p>
                              <p class="text-sm text-gray-600">
                                Reviewed at: {new Date(request.reviewed_at!).toLocaleString('ru-RU')}
                              </p>
                              <button
                                onClick={handleRequestReview}
                                disabled={requesting()}
                                class="mt-3 bg-blue-600 hover:bg-blue-700 disabled:bg-blue-400 text-white px-4 py-2 rounded"
                              >
                                {requesting() ? 'Requesting...' : 'Request Another Review'}
                              </button>
                            </div>
                          </Show>
                        </div>
                      );
                    }}
                  </Show>
                </Show>
              </div>
            </div>
          </div>
        )}
      </Show>
    </div>
  );
};

export default StudentSubmission;
