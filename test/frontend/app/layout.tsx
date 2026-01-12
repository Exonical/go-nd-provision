import type { Metadata } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import Link from "next/link";
import "./globals.css";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: "go-nd API Tester",
  description: "Test frontend for go-nd REST API",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className="dark">
      <body className={`${geistSans.variable} ${geistMono.variable} antialiased bg-background text-foreground`}>
        <nav className="bg-card border-b border-border">
          <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
            <div className="flex items-center justify-between h-16">
              <div className="flex items-center gap-8">
                <Link href="/" className="text-xl font-bold text-foreground">
                  go-nd
                </Link>
                <div className="flex gap-4">
                  <Link href="/fabrics" className="text-muted-foreground hover:text-foreground transition-colors">
                    Fabrics
                  </Link>
                  <Link href="/compute-nodes" className="text-muted-foreground hover:text-foreground transition-colors">
                    Compute Nodes
                  </Link>
                  <Link href="/interfaces" className="text-muted-foreground hover:text-foreground transition-colors">
                    Interfaces
                  </Link>
                  <Link href="/switch-ports" className="text-muted-foreground hover:text-foreground transition-colors">
                    Switch Ports
                  </Link>
                  <Link href="/storage-tenants" className="text-muted-foreground hover:text-foreground transition-colors">
                    Storage Tenants
                  </Link>
                </div>
              </div>
            </div>
          </div>
        </nav>
        <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
          {children}
        </main>
      </body>
    </html>
  );
}
