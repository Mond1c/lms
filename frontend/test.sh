cat > src/pages/Home.tsx << 'EOF'
import { Component } from 'solid-js';
import { A } from '@solidjs/router';

const Home: Component = () => {
  return (
    <div class="container mx-auto px-4 py-16">
      <div class="text-center">
        <h1 class="text-5xl font-bold mb-6">Gitea Classroom</h1>
        <p class="text-xl text-gray-600 mb-8">
          GitHub Classroom alternative for Gitea/Forgejo
        </p>
        <div class="flex gap-4 justify-center">
          <A
            href="/dashboard"
            class="bg-blue-600 hover:bg-blue-700 text-white px-8 py-3 rounded-lg text-lg"
          >
            Get Started
          </A>
        </div>
      </div>

      <div class="mt-16 grid grid-cols-1 md:grid-cols-3 gap-8">
        <div class="text-center">
          <div class="text-4xl mb-4">📚</div>
          <h3 class="text-xl font-bold mb-2">Course Management</h3>
          <p class="text-gray-600">Create and manage courses with ease</p>
        </div>
        <div class="text-center">
          <div class="text-4xl mb-4">🤖</div>
          <h3 class="text-xl font-bold mb-2">Auto Grading</h3>
          <p class="text-gray-600">Automated testing with CI/CD integration</p>
        </div>
        <div class="text-center">
          <div class="text-4xl mb-4">👥</div>
          <h3 class="text-xl font-bold mb-2">Student Management</h3>
          <p class="text-gray-600">Track progress and manage submissions</p>
        </div>
      </div>
    </div>
  );
};

export default Home;
EOF
