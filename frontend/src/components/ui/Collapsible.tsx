import React, {ReactNode, useState} from 'react';

interface CollapsibleProps {
    title: string;
    children: ReactNode;
    defaultCollapsed?: boolean;
}

const Collapsible: React.FC<CollapsibleProps> = ({title, children, defaultCollapsed = true}) => {
    const [isCollapsed, setIsCollapsed] = useState(defaultCollapsed);

    const toggleCollapsed = () => {
        setIsCollapsed(!isCollapsed);
    };

    return (
        <div className="border border-gray-200 rounded-md mb-4">
            <button
                type="button"
                className="flex justify-between items-center w-full p-4 text-left font-medium text-gray-700 bg-gray-50 hover:bg-gray-100 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2"
                onClick={toggleCollapsed}
            >
                <span>{title}</span>
                <span>{isCollapsed ? '▼' : '▲'}</span>
            </button>
            {!isCollapsed && (
                <div className="p-4 border-t border-gray-200">
                    {children}
                </div>
            )}
        </div>
    );
};

export default Collapsible;
