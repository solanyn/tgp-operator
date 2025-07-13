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
        
        # Go tools that need to be installed via go install
        goTools = pkgs.buildEnv {
          name = "go-tools";
          paths = [
            (pkgs.buildGoModule rec {
              pname = "controller-gen";
              version = "0.13.0";
              src = pkgs.fetchFromGitHub {
                owner = "kubernetes-sigs";
                repo = "controller-tools";
                rev = "v${version}";
                sha256 = "sha256-9VeMAiD3IJq8/hmpb6R7wU8EA3vIYiEbQcSP/UNxv2M=";
              };
              vendorHash = "sha256-YLQ/BN3NCbOWw6kULGaEB6RY6LdAY1Bz+2YHx2vv8qU=";
              subPackages = [ "cmd/controller-gen" ];
            })
            
            (pkgs.buildGoModule rec {
              pname = "setup-envtest";
              version = "0.0.0-20231102163445-5068bb2bb11e";
              src = pkgs.fetchFromGitHub {
                owner = "kubernetes-sigs";
                repo = "controller-runtime";
                rev = "5068bb2bb11e9e7ab3f1d34d29e9e551b8b49f36";
                sha256 = "sha256-lz8r0xF3qN9qLLEGMvRsb9QLnDW+e2Q6wL6sRzLhiTQ=";
              };
              vendorHash = "sha256-F9eJxLX4w5UeOQ6ShrVVWvDBpM8g9sY9HFjfW8lQBAg=";
              subPackages = [ "tools/setup-envtest" ];
            })
          ];
        };
      in
      {
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            # Core development tools
            go_1_21
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
            
            # Custom Go tools
            goTools
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
            echo "Release commands:"
            echo "  task release:next-version      # Preview next version"
            echo "  task release:preview-changelog # Preview changelog"
            echo "  task release:release           # Create full release"
            echo ""
            echo "All development tools are now available!"
          '';
        };
      });
}