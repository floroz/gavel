import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone",
  experimental: {
    serverActions: {
      allowedOrigins: ["app.gavel.local", "localhost:3000"],
    },
  },
};

export default nextConfig;
