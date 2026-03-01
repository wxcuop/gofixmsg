#!/usr/bin/env python3
"""
Test coverage analysis and reporting for PyFixMsg Plus.
Generates comprehensive coverage reports with gap analysis.
"""
import subprocess
import sys
import os
import json
import xml.etree.ElementTree as ET
from pathlib import Path
from typing import Dict, List, Tuple


class CoverageAnalyzer:
    """Analyzes and reports test coverage for PyFixMsg Plus."""
    
    def __init__(self, source_dir: str = "pyfixmsg_plus"):
        self.source_dir = source_dir
        self.coverage_data = {}
        self.coverage_report = {}
    
    def run_coverage_analysis(self, test_paths: List[str] = None) -> Dict:
        """Run comprehensive coverage analysis."""
        test_paths = test_paths or ["tests/"]
        
        print("Running coverage analysis...")
        
        # Run tests with coverage
        cmd = [
            "python", "-m", "pytest",
            "--cov=" + self.source_dir,
            "--cov-report=xml:coverage.xml",
            "--cov-report=html:htmlcov",
            "--cov-report=term-missing",
            "--cov-branch",  # Include branch coverage
            "-v"
        ] + test_paths
        
        try:
            result = subprocess.run(cmd, capture_output=True, text=True, timeout=1800)
            
            print(f"Coverage command completed with return code: {result.returncode}")
            if result.stdout:
                print("STDOUT:", result.stdout[-500:])  # Last 500 chars
            if result.stderr:
                print("STDERR:", result.stderr[-500:])  # Last 500 chars
            
            if result.returncode != 0:
                print(f"Coverage analysis failed with return code {result.returncode}")
                return {}
            
            # Parse coverage results
            self._parse_coverage_xml()
            self._analyze_coverage_gaps()
            
            return self.coverage_report
            
        except subprocess.TimeoutExpired:
            print("Coverage analysis timed out")
            return {}
        except Exception as e:
            print(f"Coverage analysis failed: {e}")
            return {}
    
    def _parse_coverage_xml(self):
        """Parse coverage XML report."""
        coverage_file = Path("coverage.xml")
        if not coverage_file.exists():
            print("Coverage XML file not found")
            return
        
        try:
            tree = ET.parse(coverage_file)
            root = tree.getroot()
            
            # Overall statistics
            self.coverage_report['overall'] = {
                'line_rate': float(root.get('line-rate', 0)) * 100,
                'branch_rate': float(root.get('branch-rate', 0)) * 100,
                'lines_covered': int(root.get('lines-covered', 0)),
                'lines_valid': int(root.get('lines-valid', 0)),
                'branches_covered': int(root.get('branches-covered', 0)),
                'branches_valid': int(root.get('branches-valid', 0))
            }
            
            # Package-level coverage
            packages = {}
            for package in root.findall('.//package'):
                package_name = package.get('name')
                packages[package_name] = {
                    'line_rate': float(package.get('line-rate', 0)) * 100,
                    'branch_rate': float(package.get('branch-rate', 0)) * 100,
                    'classes': {}
                }
                
                # Class-level coverage
                for class_elem in package.findall('.//class'):
                    class_name = class_elem.get('name')
                    filename = class_elem.get('filename')
                    
                    packages[package_name]['classes'][class_name] = {
                        'filename': filename,
                        'line_rate': float(class_elem.get('line-rate', 0)) * 100,
                        'branch_rate': float(class_elem.get('branch-rate', 0)) * 100,
                        'lines': {},
                        'missing_lines': []
                    }
                    
                    # Line-level coverage
                    for line in class_elem.findall('.//line'):
                        line_num = int(line.get('number'))
                        hits = int(line.get('hits'))
                        
                        packages[package_name]['classes'][class_name]['lines'][line_num] = {
                            'hits': hits,
                            'covered': hits > 0
                        }
                        
                        if hits == 0:
                            packages[package_name]['classes'][class_name]['missing_lines'].append(line_num)
            
            self.coverage_report['packages'] = packages
            
        except Exception as e:
            print(f"Error parsing coverage XML: {e}")
    
    def _analyze_coverage_gaps(self):
        """Analyze coverage gaps and identify areas needing attention."""
        gaps = {
            'low_coverage_files': [],
            'uncovered_functions': [],
            'missing_test_areas': [],
            'recommendations': []
        }
        
        # Identify low coverage files
        for package_name, package_data in self.coverage_report.get('packages', {}).items():
            for class_name, class_data in package_data.get('classes', {}).items():
                line_rate = class_data.get('line_rate', 0)
                if line_rate < 80:  # Less than 80% coverage
                    gaps['low_coverage_files'].append({
                        'file': class_data.get('filename', class_name),
                        'coverage': line_rate,
                        'missing_lines': len(class_data.get('missing_lines', []))
                    })
        
        # Sort by lowest coverage first
        gaps['low_coverage_files'].sort(key=lambda x: x['coverage'])
        
        # Generate recommendations
        overall_coverage = self.coverage_report.get('overall', {}).get('line_rate', 0)
        
        if overall_coverage < 95:
            gaps['recommendations'].append(
                f"Overall coverage is {overall_coverage:.1f}%. Target is 95%. "
                f"Focus on the {len(gaps['low_coverage_files'])} files with low coverage."
            )
        
        if gaps['low_coverage_files']:
            worst_file = gaps['low_coverage_files'][0]
            gaps['recommendations'].append(
                f"Highest priority: {worst_file['file']} has only {worst_file['coverage']:.1f}% coverage. "
                f"Add tests for {worst_file['missing_lines']} uncovered lines."
            )
        
        # Identify missing test areas based on source code structure
        self._identify_missing_test_areas(gaps)
        
        self.coverage_report['gaps'] = gaps
    
    def _identify_missing_test_areas(self, gaps: Dict):
        """Identify areas that likely need more testing based on source structure."""
        source_path = Path(self.source_dir)
        
        if not source_path.exists():
            return
        
        # Look for Python files that might not have corresponding tests
        for py_file in source_path.rglob("*.py"):
            if py_file.name == "__init__.py":
                continue
            
            relative_path = py_file.relative_to(source_path)
            test_file_patterns = [
                f"tests/unit/test_{py_file.stem}.py",
                f"tests/integration/test_{py_file.stem}.py",
                f"tests/test_{py_file.stem}.py"
            ]
            
            # Check if any corresponding test file exists
            has_test = any(Path(pattern).exists() for pattern in test_file_patterns)
            
            if not has_test:
                gaps['missing_test_areas'].append({
                    'source_file': str(relative_path),
                    'suggested_test_files': test_file_patterns
                })
    
    def generate_coverage_report(self, output_file: str = None):
        """Generate a comprehensive coverage report."""
        if not self.coverage_report:
            print("No coverage data available. Run coverage analysis first.")
            return
        
        report_lines = []
        
        # Header
        report_lines.extend([
            "=" * 80,
            "PYFIXMSG PLUS - TEST COVERAGE REPORT",
            "=" * 80,
            ""
        ])
        
        # Overall statistics
        overall = self.coverage_report.get('overall', {})
        report_lines.extend([
            "OVERALL COVERAGE STATISTICS",
            "-" * 40,
            f"Line Coverage:   {overall.get('line_rate', 0):.1f}%",
            f"Branch Coverage: {overall.get('branch_rate', 0):.1f}%",
            f"Lines Covered:   {overall.get('lines_covered', 0):,} / {overall.get('lines_valid', 0):,}",
            f"Branches Covered: {overall.get('branches_covered', 0):,} / {overall.get('branches_valid', 0):,}",
            ""
        ])
        
        # Coverage status
        line_rate = overall.get('line_rate', 0)
        if line_rate >= 95:
            status = "EXCELLENT"
        elif line_rate >= 85:
            status = "GOOD"
        elif line_rate >= 70:
            status = "NEEDS IMPROVEMENT"
        else:
            status = "POOR"
        
        report_lines.append(f"Coverage Status: {status}")
        report_lines.append("")
        
        # Package breakdown
        packages = self.coverage_report.get('packages', {})
        if packages:
            report_lines.extend([
                "PACKAGE COVERAGE BREAKDOWN",
                "-" * 40,
                f"{'Package':<30} {'Line%':<8} {'Branch%':<8}",
                "-" * 50
            ])
            
            for package_name, package_data in packages.items():
                line_rate = package_data.get('line_rate', 0)
                branch_rate = package_data.get('branch_rate', 0)
                report_lines.append(f"{package_name:<30} {line_rate:>6.1f}% {branch_rate:>7.1f}%")
            
            report_lines.append("")
        
        # Low coverage files
        gaps = self.coverage_report.get('gaps', {})
        low_coverage = gaps.get('low_coverage_files', [])
        
        if low_coverage:
            report_lines.extend([
                "FILES NEEDING ATTENTION (< 80% coverage)",
                "-" * 50,
                f"{'File':<40} {'Coverage':<10} {'Missing Lines':<12}",
                "-" * 70
            ])
            
            for file_info in low_coverage[:10]:  # Top 10 worst files
                filename = file_info['file'].split('/')[-1]  # Just filename
                coverage = file_info['coverage']
                missing = file_info['missing_lines']
                report_lines.append(f"{filename:<40} {coverage:>7.1f}% {missing:>11}")
            
            if len(low_coverage) > 10:
                report_lines.append(f"... and {len(low_coverage) - 10} more files")
            
            report_lines.append("")
        
        # Missing test areas
        missing_tests = gaps.get('missing_test_areas', [])
        if missing_tests:
            report_lines.extend([
                "SOURCE FILES WITHOUT CORRESPONDING TESTS",
                "-" * 45,
            ])
            
            for missing in missing_tests[:5]:  # Top 5
                report_lines.append(f"  {missing['source_file']}")
            
            if len(missing_tests) > 5:
                report_lines.append(f"  ... and {len(missing_tests) - 5} more files")
            
            report_lines.append("")
        
        # Recommendations
        recommendations = gaps.get('recommendations', [])
        if recommendations:
            report_lines.extend([
                "RECOMMENDATIONS",
                "-" * 20,
            ])
            
            for i, rec in enumerate(recommendations, 1):
                report_lines.append(f"{i}. {rec}")
                report_lines.append("")
        
        # Phase 2 requirements
        report_lines.extend([
            "PHASE 2 REQUIREMENTS STATUS",
            "-" * 35,
            f"Target Coverage: 95%",
            f"Current Coverage: {line_rate:.1f}%",
            f"Gap: {max(0, 95 - line_rate):.1f} percentage points",
            ""
        ])
        
        if line_rate >= 95:
            report_lines.append("✅ Phase 2 coverage requirement MET")
        else:
            needed = 95 - line_rate
            report_lines.append(f"❌ Phase 2 coverage requirement NOT MET (need {needed:.1f}% more)")
        
        report_lines.append("")
        
        # Footer
        report_lines.extend([
            "=" * 80,
            "End of Coverage Report",
            "=" * 80
        ])
        
        # Output report
        report_text = "\n".join(report_lines)
        
        if output_file:
            with open(output_file, 'w') as f:
                f.write(report_text)
            print(f"Coverage report saved to: {output_file}")
        else:
            print(report_text)
        
        return report_text
    
    def get_coverage_summary(self) -> Dict:
        """Get a simple coverage summary."""
        overall = self.coverage_report.get('overall', {})
        gaps = self.coverage_report.get('gaps', {})
        
        return {
            'line_coverage': overall.get('line_rate', 0),
            'branch_coverage': overall.get('branch_rate', 0),
            'meets_phase2_requirement': overall.get('line_rate', 0) >= 95,
            'low_coverage_files_count': len(gaps.get('low_coverage_files', [])),
            'missing_test_files_count': len(gaps.get('missing_test_areas', []))
        }


def main():
    """Main entry point for coverage analysis."""
    import argparse
    
    parser = argparse.ArgumentParser(description="PyFixMsg Plus Coverage Analyzer")
    parser.add_argument('--source', default='pyfixmsg_plus', help='Source directory to analyze')
    parser.add_argument('--tests', nargs='*', default=['tests/'], help='Test directories')
    parser.add_argument('--report', help='Output file for detailed report')
    parser.add_argument('--summary', action='store_true', help='Show summary only')
    
    args = parser.parse_args()
    
    analyzer = CoverageAnalyzer(args.source)
    
    # Run analysis
    coverage_data = analyzer.run_coverage_analysis(args.tests)
    
    if not coverage_data:
        print("Coverage analysis failed")
        sys.exit(1)
    
    if args.summary:
        # Show summary only
        summary = analyzer.get_coverage_summary()
        print(f"\nCoverage Summary:")
        print(f"  Line Coverage: {summary['line_coverage']:.1f}%")
        print(f"  Branch Coverage: {summary['branch_coverage']:.1f}%")
        print(f"  Phase 2 Requirement: {'✅ MET' if summary['meets_phase2_requirement'] else '❌ NOT MET'}")
        print(f"  Low Coverage Files: {summary['low_coverage_files_count']}")
        print(f"  Missing Test Files: {summary['missing_test_files_count']}")
    else:
        # Generate full report
        analyzer.generate_coverage_report(args.report)
    
    # Exit with appropriate code
    summary = analyzer.get_coverage_summary()
    if summary['meets_phase2_requirement']:
        sys.exit(0)
    else:
        sys.exit(1)


if __name__ == '__main__':
    main()
