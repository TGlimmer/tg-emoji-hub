import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter, Routes, Route } from "react-router-dom";
import { Toaster } from "sonner";
import App from "./App";
import { LanguageProvider } from "./i18n/LanguageProvider";
import "./index.css";

const root = document.getElementById("root");
if (!root) {
  throw new Error("#root not found in index.html");
}

createRoot(root).render(
  <StrictMode>
    <LanguageProvider>
      <BrowserRouter>
        <Routes>
          <Route path="/*" element={<App />} />
        </Routes>
      </BrowserRouter>
      <Toaster
        theme="dark"
        position="top-center"
        richColors
        closeButton={false}
        duration={1600}
      />
    </LanguageProvider>
  </StrictMode>,
);
