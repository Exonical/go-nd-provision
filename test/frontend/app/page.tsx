import Link from "next/link";
import { Card, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";

export default function Home() {
  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-3xl font-bold text-foreground">go-nd API Tester</h1>
        <p className="mt-2 text-muted-foreground">
          Test frontend for the go-nd REST API
        </p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <Link href="/fabrics">
          <Card className="hover:border-primary transition-colors cursor-pointer">
            <CardHeader>
              <CardTitle>Fabrics</CardTitle>
              <CardDescription>
                View fabrics, switches, and ports synced from Nexus Dashboard
              </CardDescription>
            </CardHeader>
          </Card>
        </Link>

        <Link href="/compute-nodes">
          <Card className="hover:border-primary transition-colors cursor-pointer">
            <CardHeader>
              <CardTitle>Compute Nodes</CardTitle>
              <CardDescription>
                Create and manage compute nodes and their port mappings
              </CardDescription>
            </CardHeader>
          </Card>
        </Link>

        <Link href="/switch-ports">
          <Card className="hover:border-primary transition-colors cursor-pointer">
            <CardHeader>
              <CardTitle>Switch Ports</CardTitle>
              <CardDescription>
                Bulk assign switch ports to compute nodes and interfaces
              </CardDescription>
            </CardHeader>
          </Card>
        </Link>

        <Link href="/interfaces">
          <Card className="hover:border-primary transition-colors cursor-pointer">
            <CardHeader>
              <CardTitle>Interfaces</CardTitle>
              <CardDescription>
                Cross-node view of compute/storage interfaces and mapping counts
              </CardDescription>
            </CardHeader>
          </Card>
        </Link>

        <Link href="/storage-tenants">
          <Card className="hover:border-primary transition-colors cursor-pointer">
            <CardHeader>
              <CardTitle>Storage Tenants</CardTitle>
              <CardDescription>
                Manage storage tenant configurations for NDFC provisioning
              </CardDescription>
            </CardHeader>
          </Card>
        </Link>
      </div>

      <Card className="bg-muted">
        <CardHeader className="py-4">
          <p className="text-sm text-muted-foreground">
            API Base URL: <code className="text-foreground font-mono">http://localhost:8080</code>
          </p>
        </CardHeader>
      </Card>
    </div>
  );
}
