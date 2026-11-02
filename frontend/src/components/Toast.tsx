import React, { createContext, useContext, useState, useCallback } from 'react';
import { Check, AlertCircle } from 'lucide-react';

interface ToastContextType {
  showToast: (msg: string, isError?: boolean) => void;
}

const ToastContext = createContext<ToastContextType>({ showToast: () => {} });

export const useToast = () => useContext(ToastContext);

export const ToastProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [toast, setToast] = useState<{ msg: string; isError: boolean } | null>(null);

  const showToast = useCallback((msg: string, isError = false) => {
    setToast({ msg, isError });
    setTimeout(() => {
      setToast(null);
    }, 2200);
  }, []);

  return (
    <ToastContext.Provider value={{ showToast }}>
      {children}
      {toast && (
        <div className="matrix-toast-center" style={{
          backgroundColor: toast.isError ? '#18181b' : '#000000',
          borderColor: toast.isError ? '#dc2626' : '#000000'
        }}>
          {toast.isError ? <AlertCircle size={15} color="#ef4444" /> : <Check size={15} color="#ffffff" />}
          <span>{toast.msg}</span>
        </div>
      )}
    </ToastContext.Provider>
  );
};
