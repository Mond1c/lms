import { type Component, createResource, createSignal, Show, For } from 'solid-js';
import { useParams, A } from '@solidjs/router';
import { courseAPI, assignmentAPI } from '../services/api';

const CoursePage: Component = () => {
  const params = useParams();
  const [showCreateAssignment, setShowCreateAssignment] = createSignal(false);
  const [copied, setCopied] = createSignal(false);

  const [course] = createResource(
    () => params.slug,
    async (slug) => {
      const response = await courseAPI.get(slug);
      return response.data;
    }
  );

  const [assignments, { refetch: refetchAssignments }] = createResource(
    () => params.slug,
    async (slug) => {
      const response = await assignmentAPI.list(slug);
      return response.data;
    }
  );

  const [formData, setFormData] = createSignal({
    title: '',
    description: '',
    template_repo: '',
    deadline: '',
    max_points: 100,
  });

  const handleCreateAssignment = async (e: Event) => {
    e.preventDefault();
    const slug = params.slug;
    if (!slug) return;
    await assignmentAPI.create(slug, formData());
    setShowCreateAssignment(false);
    setFormData({
      title: '',
      description: '',
      template_repo: '',
      deadline: '',
      max_points: 100,
    });
    refetchAssignments();
  };

  const copyInviteLink = () => {
    const c = course();
    if (c) {
      navigator.clipboard.writeText(`${window.location.origin}/join/${c.invite_code}`);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
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
    <div class="container mx-auto px-4 py-8">
      <Show
        when={!course.loading && course()}
        fallback={<div class="text-center py-8">Loading...</div>}
      >
        {(c) => (
          <>
            <div class="mb-8">
              <div class="flex justify-between items-start">
                <div>
                  <h1 class="text-3xl font-bold mb-2">{c().name}</h1>
                  <p class="text-gray-600 mb-4">{c().description}</p>
                  <div class="flex gap-4 text-sm text-gray-500">
                    <span>Organization: {c().org_name}</span>
                    <span>Students: {c().students?.length || 0}</span>
                  </div>
                </div>
                  <Show when={c().is_instructor}>
                  <div class="flex flex-col gap-2">
                    <button
                      onClick={copyInviteLink}
                      class="bg-green-600 hover:bg-green-700 text-white px-4 py-2 rounded"
                    >
                      {copied() ? 'Copied!' : 'Copy Invite Link'}
                    </button>
                    <A
                      href={`/courses/${c().slug}/students`}
                      class="bg-gray-600 hover:bg-gray-700 text-white px-4 py-2 rounded text-center"
                    >
                      Manage Students
                    </A>
                  </div>
                </Show>
              </div>
            </div>

            <div class="mb-6 flex justify-between items-center">
              <h2 class="text-2xl font-bold">Assignments</h2>
              <Show when={c().is_instructor}>
                <button
                  onClick={() => setShowCreateAssignment(true)}
                  class="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded"
                >
                  New Assignment
                </button>
              </Show>
            </div>

            <Show when={showCreateAssignment()}>
              <div class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
                <div class="bg-white rounded-lg p-6 w-full max-w-md">
                  <h3 class="text-xl font-bold mb-4">Create Assignment</h3>
                  <form onSubmit={handleCreateAssignment}>
                    <div class="mb-4">
                      <label class="block text-sm font-medium mb-1">Title</label>
                      <input
                        type="text"
                        value={formData().title}
                        onInput={(e) =>
                          setFormData({ ...formData(), title: e.currentTarget.value })
                        }
                        class="w-full border rounded px-3 py-2"
                        required
                      />
                    </div>
                    <div class="mb-4">
                      <label class="block text-sm font-medium mb-1">Description</label>
                      <textarea
                        value={formData().description}
                        onInput={(e) =>
                          setFormData({ ...formData(), description: e.currentTarget.value })
                        }
                        class="w-full border rounded px-3 py-2"
                        rows="3"
                      />
                    </div>
                    <div class="mb-4">
                      <label class="block text-sm font-medium mb-1">
                        Template Repository (optional)
                      </label>
                      <input
                        type="text"
                        value={formData().template_repo}
                        onInput={(e) =>
                          setFormData({ ...formData(), template_repo: e.currentTarget.value })
                        }
                        class="w-full border rounded px-3 py-2"
                        placeholder="owner/repo"
                      />
                    </div>
                    <div class="mb-4">
                      <label class="block text-sm font-medium mb-1">Deadline</label>
                      <input
                        type="datetime-local"
                        value={formData().deadline}
                        onInput={(e) =>
                          setFormData({ ...formData(), deadline: e.currentTarget.value })
                        }
                        class="w-full border rounded px-3 py-2"
                        required
                      />
                    </div>
                    <div class="mb-4">
                      <label class="block text-sm font-medium mb-1">Max Points</label>
                      <input
                        type="number"
                        value={formData().max_points}
                        onInput={(e) =>
                          setFormData({
                            ...formData(),
                            max_points: parseInt(e.currentTarget.value),
                          })
                        }
                        class="w-full border rounded px-3 py-2"
                        min="1"
                      />
                    </div>
                    <div class="flex justify-end gap-2">
                      <button
                        type="button"
                        onClick={() => setShowCreateAssignment(false)}
                        class="px-4 py-2 border rounded hover:bg-gray-100"
                      >
                        Cancel
                      </button>
                      <button
                        type="submit"
                        class="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded"
                      >
                        Create
                      </button>
                    </div>
                  </form>
                </div>
              </div>
            </Show>

            <Show
              when={!assignments.loading}
              fallback={<div>Loading assignments...</div>}
            >
              <Show
                when={assignments()?.length}
                fallback={
                  <div class="text-center py-8 text-gray-500">
                    No assignments yet. Create your first assignment!
                  </div>
                }
              >
                <div class="space-y-4">
                  <For each={assignments()}>
                    {(assignment) => (
                      <div class="bg-white rounded-lg shadow p-4 hover:shadow-md transition-shadow">
                        <div class="flex justify-between items-start">
                          <div>
                            <h3 class="text-lg font-semibold">{assignment.title}</h3>
                            <p class="text-gray-600 text-sm mt-1">
                              {assignment.description}
                            </p>
                          </div>
                          <div class="text-right text-sm">
                            <div class="text-gray-500">
                              Deadline: {formatDeadline(assignment.deadline)}
                            </div>
                            <div class="text-blue-600 font-medium">
                              {assignment.max_points} points
                            </div>
                          </div>
                        </div>
                        <div class="mt-3 flex gap-2">
                          <Show
                            when={c().is_instructor}
                            fallback={
                              <A
                                href={`/accept/${assignment.id}`}
                                class="bg-green-600 hover:bg-green-700 text-white px-4 py-1 rounded text-sm"
                              >
                                Accept Assignment
                              </A>
                            }
                          >
                            <A
                              href={`/assignments/${assignment.id}`}
                              class="bg-blue-600 hover:bg-blue-700 text-white px-4 py-1 rounded text-sm"
                            >
                              View Submissions
                            </A>
                          </Show>
                        </div>
                      </div>
                    )}
                  </For>
                </div>
              </Show>
            </Show>
          </>
        )}
      </Show>
    </div>
  );
};

export default CoursePage;
