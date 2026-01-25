import { type Component, createResource, createSignal, Show, For } from 'solid-js';
import { useParams, A } from '@solidjs/router';
import { assignmentAPI, submissionAPI, type Submission } from '../services/api';

const AssignmentPage: Component = () => {
  const params = useParams();
  const [gradingSubmission, setGradingSubmission] = createSignal<Submission | null>(null);
  const [gradeData, setGradeData] = createSignal({ score: 0, feedback: '' });
  const [copied, setCopied] = createSignal(false);

  const [assignment] = createResource(
    () => params.id ? parseInt(params.id) : 0,
    async (id) => {
      if (!id) return null;
      const response = await assignmentAPI.get(id);
      return response.data;
    }
  );

  const [submissions, { refetch: refetchSubmissions }] = createResource(
    () => params.id ? parseInt(params.id) : 0,
    async (id) => {
      if (!id) return [];
      const response = await submissionAPI.list(id);
      return response.data;
    }
  );

  const copyAcceptLink = () => {
    navigator.clipboard.writeText(`${window.location.origin}/accept/${params.id}`);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const handleGrade = async (e: Event) => {
    e.preventDefault();
    const sub = gradingSubmission();
    if (sub) {
      await submissionAPI.grade(sub.id, gradeData());
      setGradingSubmission(null);
      setGradeData({ score: 0, feedback: '' });
      refetchSubmissions();
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

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'graded':
        return 'bg-green-100 text-green-800';
      case 'in_progress':
        return 'bg-yellow-100 text-yellow-800';
      case 'submitted':
        return 'bg-blue-100 text-blue-800';
      default:
        return 'bg-gray-100 text-gray-800';
    }
  };

  return (
    <div class="container mx-auto px-4 py-8">
      <Show
        when={!assignment.loading && assignment()}
        fallback={<div class="text-center py-8">Loading...</div>}
      >
        {(a) => (
          <>
            <div class="mb-8">
              <A href={`/courses/${a().course_id}`} class="text-blue-600 hover:underline mb-2 inline-block">
                Back to Course
              </A>
              <h1 class="text-3xl font-bold mb-2">{a().title}</h1>
              <p class="text-gray-600 mb-4">{a().description}</p>
              <div class="flex gap-6 text-sm">
                <span class="text-gray-500">
                  Deadline: {formatDeadline(a().deadline)}
                </span>
                <span class="text-blue-600 font-medium">
                  Max Points: {a().max_points}
                </span>
                {a().template_repo && (
                  <span class="text-gray-500">
                    Template: {a().template_repo}
                  </span>
                )}
              </div>
            </div>

            <div class="flex justify-between items-center mb-6">
              <h2 class="text-2xl font-bold">Submissions</h2>
              <button
                onClick={copyAcceptLink}
                class="bg-green-600 hover:bg-green-700 text-white px-4 py-2 rounded"
              >
                {copied() ? 'Copied!' : 'Copy Assignment Link'}
              </button>
            </div>

            <Show when={gradingSubmission()}>
              <div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
                <div class="bg-white rounded-lg p-6 w-full max-w-md">
                  <h3 class="text-xl font-bold mb-4">
                    Grade Submission - {gradingSubmission()?.student?.username}
                  </h3>
                  <form onSubmit={handleGrade}>
                    <div class="mb-4">
                      <label class="block text-sm font-medium mb-1">
                        Score (max {assignment()?.max_points})
                      </label>
                      <input
                        type="number"
                        value={gradeData().score}
                        onInput={(e) =>
                          setGradeData({ ...gradeData(), score: parseInt(e.currentTarget.value) })
                        }
                        class="w-full border rounded px-3 py-2"
                        min="0"
                        max={assignment()?.max_points}
                        required
                      />
                    </div>
                    <div class="mb-4">
                      <label class="block text-sm font-medium mb-1">Feedback</label>
                      <textarea
                        value={gradeData().feedback}
                        onInput={(e) =>
                          setGradeData({ ...gradeData(), feedback: e.currentTarget.value })
                        }
                        class="w-full border rounded px-3 py-2"
                        rows="4"
                      />
                    </div>
                    <div class="flex justify-end gap-2">
                      <button
                        type="button"
                        onClick={() => setGradingSubmission(null)}
                        class="px-4 py-2 border rounded hover:bg-gray-100"
                      >
                        Cancel
                      </button>
                      <button
                        type="submit"
                        class="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded"
                      >
                        Submit Grade
                      </button>
                    </div>
                  </form>
                </div>
              </div>
            </Show>

            <Show
              when={!submissions.loading}
              fallback={<div>Loading submissions...</div>}
            >
              <Show
                when={submissions()?.length}
                fallback={
                  <div class="text-center py-8 text-gray-500">
                    No submissions yet.
                  </div>
                }
              >
                <div class="bg-white rounded-lg shadow overflow-hidden">
                  <table class="w-full">
                    <thead class="bg-gray-50">
                      <tr>
                        <th class="px-4 py-3 text-left text-sm font-medium text-gray-500">
                          Student
                        </th>
                        <th class="px-4 py-3 text-left text-sm font-medium text-gray-500">
                          Repository
                        </th>
                        <th class="px-4 py-3 text-left text-sm font-medium text-gray-500">
                          Status
                        </th>
                        <th class="px-4 py-3 text-left text-sm font-medium text-gray-500">
                          Score
                        </th>
                        <th class="px-4 py-3 text-left text-sm font-medium text-gray-500">
                          Actions
                        </th>
                      </tr>
                    </thead>
                    <tbody class="divide-y divide-gray-200">
                      <For each={submissions()}>
                        {(submission) => (
                          <tr>
                            <td class="px-4 py-3">
                              <div class="font-medium">
                                {submission.student?.full_name || submission.student?.username}
                              </div>
                              <div class="text-sm text-gray-500">
                                {submission.student?.email}
                              </div>
                            </td>
                            <td class="px-4 py-3">
                              <a
                                href={submission.repo_url}
                                target="_blank"
                                rel="noopener noreferrer"
                                class="text-blue-600 hover:underline"
                              >
                                View Repository
                              </a>
                            </td>
                            <td class="px-4 py-3">
                              <span
                                class={`px-2 py-1 rounded-full text-xs font-medium ${getStatusColor(
                                  submission.status
                                )}`}
                              >
                                {submission.status}
                              </span>
                            </td>
                            <td class="px-4 py-3">
                              {submission.score !== null
                                ? `${submission.score}/${assignment()?.max_points}`
                                : '-'}
                            </td>
                            <td class="px-4 py-3">
                              <button
                                onClick={() => {
                                  setGradingSubmission(submission);
                                  setGradeData({
                                    score: submission.score || 0,
                                    feedback: submission.feedback || '',
                                  });
                                }}
                                class="text-blue-600 hover:underline"
                              >
                                Grade
                              </button>
                            </td>
                          </tr>
                        )}
                      </For>
                    </tbody>
                  </table>
                </div>
              </Show>
            </Show>
          </>
        )}
      </Show>
    </div>
  );
};

export default AssignmentPage;
