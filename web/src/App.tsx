import { Navigate, Route, Routes } from "react-router-dom";
import Layout from "./components/Layout";
import ProtectedRoute from "./components/ProtectedRoute";
import Login from "./pages/Login";
import QueryStudio from "./pages/QueryStudio";
import { useSyncTheme } from "./hooks/useSyncTheme";

export default function App() {
  useSyncTheme();
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route element={<ProtectedRoute />}>
        <Route element={<Layout />}>
          <Route path="/query" element={<QueryStudio />} />
          <Route path="/" element={<Navigate to="/query" replace />} />
        </Route>
      </Route>
    </Routes>
  );
}
