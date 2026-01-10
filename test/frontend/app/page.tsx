import Link from "next/link";

export default function Home() {
  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-3xl font-bold text-zinc-900 dark:text-white">go-nd API Tester</h1>
        <p className="mt-2 text-zinc-600 dark:text-zinc-400">
          Test frontend for the go-nd REST API
        </p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <Link
          href="/fabrics"
          className="block p-6 bg-white dark:bg-zinc-900 rounded-lg border border-zinc-200 dark:border-zinc-800 hover:border-blue-500 dark:hover:border-blue-500 transition-colors"
        >
          <h2 className="text-xl font-semibold text-zinc-900 dark:text-white">Fabrics</h2>
          <p className="mt-2 text-zinc-600 dark:text-zinc-400">
            View fabrics, switches, and ports synced from Nexus Dashboard
          </p>
        </Link>

        <Link
          href="/compute-nodes"
          className="block p-6 bg-white dark:bg-zinc-900 rounded-lg border border-zinc-200 dark:border-zinc-800 hover:border-blue-500 dark:hover:border-blue-500 transition-colors"
        >
          <h2 className="text-xl font-semibold text-zinc-900 dark:text-white">Compute Nodes</h2>
          <p className="mt-2 text-zinc-600 dark:text-zinc-400">
            Create and manage compute nodes and their port mappings
          </p>
        </Link>

        <Link
          href="/switch-ports"
          className="block p-6 bg-white dark:bg-zinc-900 rounded-lg border border-zinc-200 dark:border-zinc-800 hover:border-blue-500 dark:hover:border-blue-500 transition-colors"
        >
          <h2 className="text-xl font-semibold text-zinc-900 dark:text-white">Switch Ports</h2>
          <p className="mt-2 text-zinc-600 dark:text-zinc-400">
            Bulk assign switch ports to compute nodes and interfaces
          </p>
        </Link>

        <Link
          href="/storage-tenants"
          className="block p-6 bg-white dark:bg-zinc-900 rounded-lg border border-zinc-200 dark:border-zinc-800 hover:border-blue-500 dark:hover:border-blue-500 transition-colors"
        >
          <h2 className="text-xl font-semibold text-zinc-900 dark:text-white">Storage Tenants</h2>
          <p className="mt-2 text-zinc-600 dark:text-zinc-400">
            Manage storage tenant configurations for NDFC provisioning
          </p>
        </Link>
      </div>

      <div className="p-4 bg-zinc-100 dark:bg-zinc-800 rounded-lg">
        <p className="text-sm text-zinc-600 dark:text-zinc-400">
          API Base URL: <code className="text-zinc-900 dark:text-white">http://localhost:8080</code>
        </p>
      </div>
    </div>
  );
}
