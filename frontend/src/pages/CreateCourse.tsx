import { type Component, createSignal, onMount } from 'solid-js';
import { useNavigate } from '@solidjs/router';
import { courseAPI } from '../services/api';
import { authStore } from '../stores/auth';

const CreateCourse: Component = () => {
  const navigate = useNavigate();
  const [name, setName] = createSignal('');
  const [description, setDescription] = createSignal('');
  const [loading, setLoading] = createSignal(false);
  const [error, setError] = createSignal('');

  onMount(() => {
    if (!authStore.user()?.is_admin) {
      navigate('/dashboard');
    }
  });

  const handleSubmit = async (e: Event) => {
    e.preventDefault();
    setLoading(true);
    setError('');

    try {
      const response = await courseAPI.create({
        name: name(),
        description: description(),
      });
      navigate(`/courses/${response.data.slug}`);
    } catch (err: any) {
      setError(err.response?.data?.message || 'Failed to create course');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div class="container mx-auto px-4 py-8 max-w-2xl">
      <h1 class="text-3xl font-bold mb-8">Create New Course</h1>

      <form onSubmit={handleSubmit} class="bg-white rounded-lg shadow-md p-6">
        <div class="mb-6">
          <label class="block text-gray-700 font-bold mb-2">
            Course Name
          </label>
          <input
            type="text"
            value={name()}
            onInput={(e) => setName(e.currentTarget.value)}
            class="w-full px-4 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
            placeholder="Introduction to Programming"
            required
          />
        </div>

        <div class="mb-6">
          <label class="block text-gray-700 font-bold mb-2">
            Description
          </label>
          <textarea
            value={description()}
            onInput={(e) => setDescription(e.currentTarget.value)}
            class="w-full px-4 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
            rows="4"
            placeholder="Learn the fundamentals of programming..."
          />
        </div>

        {error() && (
          <div class="mb-6 p-4 bg-red-100 text-red-700 rounded">
            {error()}
          </div>
        )}

        <div class="flex gap-4">
          <button
            type="submit"
            disabled={loading()}
            class="bg-blue-600 hover:bg-blue-700 text-white px-6 py-2 rounded disabled:opacity-50"
          >
            {loading() ? 'Creating...' : 'Create Course'}
          </button>
          <button
            type="button"
            onClick={() => navigate('/dashboard')}
            class="bg-gray-300 hover:bg-gray-400 text-gray-800 px-6 py-2 rounded"
          >
            Cancel
          </button>
        </div>
      </form>
    </div>
  );
};

export default CreateCourse;