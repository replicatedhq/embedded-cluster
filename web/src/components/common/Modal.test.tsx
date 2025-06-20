import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { Modal } from './Modal';

describe('Modal', () => {
  it('renders when component is mounted', () => {
    render(
      <Modal onClose={vi.fn()} title="Test Modal">
        <p>Modal content</p>
      </Modal>
    );
    
    expect(screen.getByText('Test Modal')).toBeInTheDocument();
    expect(screen.getByText('Modal content')).toBeInTheDocument();
  });

  it('does not render when conditionally excluded by parent', () => {
    const shouldShow = false;
    render(
      <div>
        {shouldShow && (
          <Modal onClose={vi.fn()} title="Test Modal">
            <p>Modal content</p>
          </Modal>
        )}
      </div>
    );
    
    expect(screen.queryByText('Test Modal')).not.toBeInTheDocument();
    expect(screen.queryByText('Modal content')).not.toBeInTheDocument();
  });

  it('renders when conditionally included by parent', () => {
    const shouldShow = true;
    render(
      <div>
        {shouldShow && (
          <Modal onClose={vi.fn()} title="Test Modal">
            <p>Modal content</p>
          </Modal>
        )}
      </div>
    );
    
    expect(screen.getByText('Test Modal')).toBeInTheDocument();
    expect(screen.getByText('Modal content')).toBeInTheDocument();
  });

  it('calls onClose when close button is clicked', () => {
    const onClose = vi.fn();
    render(
      <Modal onClose={onClose} title="Test Modal">
        <p>Modal content</p>
      </Modal>
    );
    
    const closeButton = screen.getByRole('button', { name: /close/i });
    fireEvent.click(closeButton);
    
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('calls onClose when background overlay is clicked', () => {
    const onClose = vi.fn();
    render(
      <Modal onClose={onClose} title="Test Modal">
        <p>Modal content</p>
      </Modal>
    );
    
    const overlay = document.querySelector('.bg-gray-500');
    fireEvent.click(overlay!);
    
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('renders footer when provided', () => {
    render(
      <Modal 
        onClose={vi.fn()} 
        title="Test Modal"
        footer={<button>Footer Button</button>}
      >
        <p>Modal content</p>
      </Modal>
    );
    
    expect(screen.getByText('Footer Button')).toBeInTheDocument();
  });
}); 