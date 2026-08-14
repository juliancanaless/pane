class Pane < Formula
  desc "Shared local memory and coordination for concurrent coding agents"
  homepage "https://github.com/juliancanaless/pane"
  version "0.1.8"
  license "MIT"
  head "https://github.com/juliancanaless/pane.git", branch: "main"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/juliancanaless/pane/releases/download/v0.1.8/pane-v0.1.8-darwin-arm64.tar.gz"
      sha256 "4f73f483713b90c9cec9f9cfea1ad58c9633db58ce8205c2eb6a0687c900039e"
    else
      url "https://github.com/juliancanaless/pane/releases/download/v0.1.8/pane-v0.1.8-darwin-amd64.tar.gz"
      sha256 "98b39c0b3b271fea69db8d72264c859a8b918d9ce0c1a85967bf560c5262de08"
    end
  end

  depends_on "go" => :build if build.head?
  depends_on "rust" => :build if build.head?

  def install
    if build.head?
      ENV.prepend_path "PATH", Formula["rust"].opt_bin
      system "make", "build"
      bin.install "bin/pane"
      bin.install "bin/pane-analyze"
    else
      bin.install "pane"
      bin.install "pane-analyze"
    end
  end

  test do
    assert_match "Pane gives concurrent coding agents", shell_output("#{bin}/pane help")
    assert_match "platform:", shell_output("#{bin}/pane doctor")
  end
end
