{
  description = "TGP Operator - Kubernetes operator for ephemeral GPU provisioning";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        
      in
      {
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            # Core development tools
            go
            go-task
            git
            
            # Container and Kubernetes tools
            docker
            helm
            kubectl
            
            # CLI tools
            github-cli
            yq-go
            jq
            
            # Go development tools
            golangci-lint
            gofumpt
            
            # Linting and formatting tools
            kubeconform
            svu
            git-chglog
            yamllint
            yamlfmt
            markdownlint-cli
            shellcheck
            shfmt
            nixpkgs-fmt
            actionlint
          ];

          shellHook = ''
            echo "🚀 TGP Operator development environment ready!"
            echo ""
            echo "Getting started:"
            echo "  task setup       # Initialize git hooks and test environment"
            echo "  task dev:build   # Build the operator"
            echo "  task test:all    # Run all tests"
            echo "  task ci          # Run CI workflow"
            echo ""
            echo "Note: Some Go tools (controller-gen, setup-envtest) will be"
            echo "automatically installed via 'go install' when first used."
            echo ""
            echo "All development tools are now available!"
          '';
        };
      });
}