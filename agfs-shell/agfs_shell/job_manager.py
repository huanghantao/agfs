"""Background job management for agfs-shell"""

import threading
import time
from typing import Dict, Optional, List
from dataclasses import dataclass, field
from enum import Enum


class JobState(Enum):
    """State of a background job"""
    RUNNING = "Running"
    COMPLETED = "Done"
    FAILED = "Failed"


@dataclass
class Job:
    """Represents a background job"""
    job_id: int
    command: str
    thread: threading.Thread
    state: JobState
    exit_code: Optional[int] = None
    start_time: float = field(default_factory=time.time)
    end_time: Optional[float] = None

    def is_alive(self) -> bool:
        """Check if job thread is still running"""
        return self.thread.is_alive()


class JobManager:
    """Manages background jobs in a thread-safe manner"""

    def __init__(self):
        self.jobs: Dict[int, Job] = {}
        self.next_job_id = 1
        self._lock = threading.Lock()
        self._notified_jobs: set = set()  # Track which jobs have been notified

    def add_job(self, command: str, thread: threading.Thread) -> int:
        """Add a new background job and return its job ID"""
        with self._lock:
            job_id = self.next_job_id
            self.next_job_id += 1

            job = Job(
                job_id=job_id,
                command=command,
                thread=thread,
                state=JobState.RUNNING
            )
            self.jobs[job_id] = job
            return job_id

    def update_job_status(self, job_id: int, exit_code: int):
        """Update job status when it completes"""
        with self._lock:
            if job_id in self.jobs:
                job = self.jobs[job_id]
                job.exit_code = exit_code
                job.end_time = time.time()
                job.state = JobState.COMPLETED if exit_code == 0 else JobState.FAILED

    def get_job(self, job_id: int) -> Optional[Job]:
        """Get job by ID"""
        with self._lock:
            return self.jobs.get(job_id)

    def get_running_jobs(self) -> List[Job]:
        """Get list of currently running jobs"""
        self._reap_completed_jobs()
        with self._lock:
            return [job for job in self.jobs.values() if job.state == JobState.RUNNING]

    def get_all_jobs(self) -> List[Job]:
        """Get all jobs (running and completed)"""
        self._reap_completed_jobs()
        with self._lock:
            return list(self.jobs.values())

    def _reap_completed_jobs(self):
        """Update status of completed jobs that haven't been reaped yet"""
        with self._lock:
            for job in self.jobs.values():
                if job.state == JobState.RUNNING and not job.is_alive():
                    # Thread completed but status not updated yet
                    # This means the job finished without calling update_job_status
                    # (likely due to exception or early termination)
                    job.state = JobState.COMPLETED
                    job.end_time = time.time()
                    if job.exit_code is None:
                        job.exit_code = 0

    def remove_job(self, job_id: int):
        """Remove a job from the manager"""
        with self._lock:
            if job_id in self.jobs:
                del self.jobs[job_id]

    def wait_for_job(self, job_id: int) -> Optional[int]:
        """Wait for specific job to complete and return its exit code"""
        job = self.get_job(job_id)
        if not job:
            return None

        # Wait for thread to complete (thread object is immutable, safe to access)
        job.thread.join()

        # After join, read exit_code with lock protection
        # Job might have been removed, so re-fetch
        with self._lock:
            if job_id in self.jobs:
                return self.jobs[job_id].exit_code
            else:
                # Job was removed, but we still have the reference
                return job.exit_code

    def wait_for_all(self):
        """Wait for all jobs to complete"""
        jobs_to_wait = self.get_running_jobs()
        for job in jobs_to_wait:
            job.thread.join()

    def cleanup_finished_jobs(self):
        """Remove completed jobs that have been notified (like bash behavior)"""
        with self._lock:
            # Only remove jobs that are completed AND have been notified
            notified_completed_ids = [
                job_id for job_id, job in self.jobs.items()
                if job.state in (JobState.COMPLETED, JobState.FAILED)
                and job_id in self._notified_jobs
                and not job.is_alive()
            ]
            for job_id in notified_completed_ids:
                del self.jobs[job_id]
                # Also remove from notified set
                self._notified_jobs.discard(job_id)

    def get_unnotified_completed_jobs(self) -> List[Job]:
        """Get completed jobs that haven't been notified yet"""
        with self._lock:
            result = []
            for job in self.jobs.values():
                if (job.state in (JobState.COMPLETED, JobState.FAILED) and
                    job.job_id not in self._notified_jobs):
                    result.append(job)
            return result

    def mark_job_notified(self, job_id: int):
        """Mark a job as notified"""
        with self._lock:
            self._notified_jobs.add(job_id)
