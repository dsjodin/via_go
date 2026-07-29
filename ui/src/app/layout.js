import "./globals.css";
import Shell from "@/components/Shell";

export const metadata = {
  title: "go-via",
  description: "ESXi imaging appliance",
};

export default function RootLayout({ children }) {
  return (
    <html lang="en">
      <body className="min-h-screen bg-slate-950 text-slate-200 antialiased">
        <Shell>{children}</Shell>
      </body>
    </html>
  );
}
